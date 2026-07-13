package proxy

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"sync"
	"time"

	"github.com/weiliang79/belune/internal/naming"
	"github.com/weiliang79/belune/internal/status"
	"github.com/weiliang79/belune/internal/store/generated"
)

// Reconciler periodically compares DB-declared routes against Caddy's live
// route table and reconciles any drift by adding missing routes and removing
// stale ones. It replaces the one-shot recoverProxyRoutes startup call.
type Reconciler struct {
	queries  *generated.Queries
	proxy    ProxyManager
	dec      Decryptor
	interval time.Duration

	// net re-joins the proxy container to each project's network. Optional: nil
	// when the proxy is not a container we manage (Caddy on the host), in which
	// case the attachment is somebody else's problem.
	net            NetworkAttacher
	proxyContainer string

	// runMu serialises whole reconcile passes so a manual ReconcileNow can't
	// overlap the periodic tick (reconcile is diff-based/idempotent but issues
	// concurrent Caddy add/remove calls otherwise).
	runMu sync.Mutex

	mu     sync.RWMutex
	status ReconcilerStatus
}

// ReconcilerStatus is a snapshot of the reconciler's recent activity. It is
// exposed over the admin API so operators can tell at a glance whether the
// background loop is still healthy and whether Caddy has been drifting.
type ReconcilerStatus struct {
	IntervalSeconds int       `json:"interval_seconds"`
	LastRunAt       time.Time `json:"last_run_at"`
	LastDurationMs  int64     `json:"last_duration_ms"`
	LastAdded       int       `json:"last_added"`
	// LastUpdated counts routes that existed but did not match the database, and
	// had to be rewritten — drift a presence check would never have seen.
	LastUpdated int    `json:"last_updated"`
	LastRemoved int    `json:"last_removed"`
	LastError   string `json:"last_error,omitempty"`
	RunCount    int64  `json:"run_count"`
	// TotalDrift counts every route that had to be added or removed across
	// all runs. Stable value over time means Caddy is in sync with the DB.
	TotalDrift int64 `json:"total_drift"`
}

// NewReconciler creates a Reconciler with the given tick interval. dec unwraps
// the stored certificate PEM for domains serving an uploaded certificate.
func NewReconciler(queries *generated.Queries, proxy ProxyManager, dec Decryptor, interval time.Duration) *Reconciler {
	return &Reconciler{
		queries:  queries,
		proxy:    proxy,
		dec:      dec,
		interval: interval,
		status:   ReconcilerStatus{IntervalSeconds: int(interval / time.Second)},
	}
}

// WithNetworkAttacher tells the reconciler to keep the proxy container joined to
// every project network it needs, naming the proxy container to attach.
func (r *Reconciler) WithNetworkAttacher(net NetworkAttacher, proxyContainer string) *Reconciler {
	r.net = net
	r.proxyContainer = proxyContainer
	return r
}

// Status returns a snapshot of the reconciler's latest run state.
func (r *Reconciler) Status() ReconcilerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// ReconcileNow runs a reconciliation pass on demand (the admin "reconcile
// routes" action) and returns the pass's first error, if any. It shares the run
// mutex with the periodic loop, so a manual trigger cannot overlap a tick.
func (r *Reconciler) ReconcileNow(ctx context.Context) error {
	r.reconcile(ctx)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.status.LastError != "" {
		return errors.New(r.status.LastError)
	}
	return nil
}

// Run starts the reconcile loop. It reconciles immediately on entry, then on
// every tick. Returns when ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	r.reconcile(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// reconcile performs one reconciliation pass. Errors are logged and retried on
// the next tick — reconciliation is best-effort and must not block startup.
// The outcome (added/removed counts, first error) is recorded on the
// Reconciler so it can be surfaced via Status().
func (r *Reconciler) reconcile(ctx context.Context) {
	r.runMu.Lock()
	defer r.runMu.Unlock()

	start := time.Now()
	var added, removed, updated int
	var firstErr error
	recordErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	defer func() {
		r.mu.Lock()
		r.status.LastRunAt = start
		r.status.LastDurationMs = time.Since(start).Milliseconds()
		r.status.LastAdded = added
		r.status.LastUpdated = updated
		r.status.LastRemoved = removed
		r.status.RunCount++
		r.status.TotalDrift += int64(added + removed + updated)
		if firstErr != nil {
			r.status.LastError = firstErr.Error()
		} else {
			r.status.LastError = ""
		}
		r.mu.Unlock()
	}()

	expected, networks, err := r.buildExpected(ctx)
	if err != nil {
		slog.Warn("proxy reconciler: failed to build expected routes", "error", err)
		recordErr(err)
		return
	}

	// Before the routes: a route pointing at a container the proxy cannot resolve
	// is a 502, and a recreated proxy container has lost every project network it
	// ever joined.
	r.ensureNetworks(ctx, networks)

	// Re-assert the server-level config (HTTPS listener, auto-HTTPS policy, skip
	// list) before touching routes. A restarted Caddy comes back up with only its
	// static Caddyfile, so this is what restores the :443 listener — and doing it
	// first means a route re-added below cannot briefly trigger a doomed
	// certificate attempt for an ssl_mode=off hostname.
	// The dashboard has no domains row, so its TLS choice has to be folded into
	// these lists by hand. Miss it out and the mode is decorative: an off
	// dashboard would still have a certificate obtained for it, and a custom one
	// would be served Caddy's internal cert in preference to the uploaded one.
	dashboardHost, dashboardMode, dashboardCertID := r.dashboardTLS(ctx)

	var skip, skipCerts []string
	for _, cfg := range expected {
		switch cfg.SSLMode {
		case SSLModeOff:
			skip = append(skip, cfg.Hostname)
		case SSLModeCustom:
			skipCerts = append(skipCerts, cfg.Hostname)
		}
	}
	if dashboardHost != "" {
		switch dashboardMode {
		case SSLModeOff:
			skip = append(skip, dashboardHost)
		case SSLModeCustom:
			skipCerts = append(skipCerts, dashboardHost)
		}
	}
	if err := r.proxy.SyncAutoHTTPS(ctx, skip, skipCerts); err != nil {
		slog.Warn("proxy reconciler: failed to sync auto-HTTPS config", "error", err)
		recordErr(err)
	}

	// Uploaded certificates are held in Caddy's memory, so a restart drops them
	// just as it drops routes. Re-assert the whole set from the DB each pass.
	if err := r.syncCertificates(ctx, dashboardHost, dashboardMode, dashboardCertID); err != nil {
		slog.Warn("proxy reconciler: failed to sync certificates", "error", err)
		recordErr(err)
	}

	// The dashboard's own route, likewise. It is not backed by a domains row, so
	// it is both re-asserted here and exempted from the stale-route sweep below —
	// otherwise the sweep would see a host-matched route with no matching domain
	// and delete the dashboard within one interval.
	if err := r.proxy.SetDashboardRoute(ctx, dashboardHost, dashboardMode); err != nil {
		slog.Warn("proxy reconciler: failed to sync dashboard route", "error", err)
		recordErr(err)
	}

	current, err := r.proxy.ListRoutes(ctx)
	if err != nil {
		slog.Warn("proxy reconciler: failed to list current routes", "error", err)
		recordErr(err)
		return
	}

	// Index current routes by host *and* path. Keying on hostname alone would
	// collapse shop.com/ and shop.com/api into one entry, so the reconciler would
	// see a single route where there are two: it would treat whichever it kept as
	// covering both, and delete the other as stale on the next pass.
	currentSet := make(map[routeKey]struct{}, len(current))
	for _, c := range current {
		currentSet[keyFor(c)] = struct{}{}
	}

	// Index expected routes by host + path, for the same reason.
	expectedSet := make(map[routeKey]RouteConfig, len(expected))
	for _, e := range expected {
		expectedSet[keyFor(e)] = e
	}

	// Make every expected route match the database — not merely exist.
	//
	// This used to add only the routes Caddy was missing, which quietly assumed a
	// route with the right hostname had the right configuration. It need not: a
	// domain update writes the database and then pushes the route, and if that push
	// fails the handler returns 200 anyway and leaves the old route behind. The
	// hostname is still present, so a presence check finds nothing wrong and the
	// domain serves stale config for ever.
	for key, cfg := range expectedSet {
		_, existed := currentSet[key]
		changed, err := r.proxy.EnsureRoute(ctx, cfg)
		switch {
		case err != nil:
			slog.Warn("proxy reconciler: failed to ensure route", "hostname", key.hostname, "path", key.path, "error", err)
			recordErr(err)
		case changed && !existed:
			slog.Info("proxy reconciler: restored missing route", "hostname", key.hostname, "path", key.path)
			added++
		case changed:
			slog.Info("proxy reconciler: repaired drifted route", "hostname", key.hostname, "path", key.path)
			updated++
		}
	}

	// Remove routes in Caddy that no longer exist in DB.
	for key := range currentSet {
		if key.hostname == dashboardHost && dashboardHost != "" {
			continue // Belune's own dashboard: no domains row backs it.
		}
		if _, exists := expectedSet[key]; !exists {
			if err := r.proxy.RemoveRoute(ctx, key.hostname, key.path); err != nil {
				slog.Warn("proxy reconciler: failed to remove stale route", "hostname", key.hostname, "path", key.path, "error", err)
				recordErr(err)
			} else {
				slog.Info("proxy reconciler: removed stale route", "hostname", key.hostname, "path", key.path)
				removed++
			}
		}
	}

	if added > 0 || removed > 0 {
		slog.Info("proxy reconciler: reconciliation complete", "added", added, "removed", removed)
	}
}

// buildExpected returns the full set of RouteConfigs that should exist in
// Caddy, derived from all running applications and their domains in the DB.
// buildExpected returns the routes the proxy should be serving, and the project
// networks the proxy must be joined to in order to reach them.
// routeKey identifies a route the way Caddy does now: a hostname no longer names
// one route, only a (hostname, path) pair does.
type routeKey struct {
	hostname string
	path     string
}

// keyFor normalises the empty path to "/", so a config that predates paths and
// one that explicitly says "root" are the same key rather than two.
func keyFor(cfg RouteConfig) routeKey {
	path := cfg.Path
	if path == "" {
		path = "/"
	}
	return routeKey{hostname: cfg.Hostname, path: path}
}

func (r *Reconciler) buildExpected(ctx context.Context) ([]RouteConfig, []string, error) {
	apps, err := r.queries.ListAllApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	var configs []RouteConfig
	networks := make(map[string]struct{})
	for _, app := range apps {
		if app.Status != status.ApplicationRunning {
			continue
		}

		row, err := r.queries.GetApplicationWithProjectSlug(ctx, app.ID)
		if err != nil {
			slog.Warn("proxy reconciler: failed to get application slug", "error", err)
			continue
		}

		// naming.ContainerName returns appSlug directly (the DB slug is the full container name).
		containerName := naming.ContainerName(row.ProjectSlug, row.Slug, "")

		domains, err := r.queries.ListDomainsByApplication(ctx, app.ID)
		if err != nil {
			slog.Warn("proxy reconciler: failed to list domains", "error", err)
			continue
		}

		for _, domain := range domains {
			cfg, err := BuildRouteConfigFromDB(ctx, r.queries, r.dec, domain, containerName, app.HealthCheckPath.String)
			if err != nil {
				slog.Warn("proxy reconciler: failed to build route config", "hostname", domain.Hostname, "error", err)
				continue
			}
			configs = append(configs, cfg)
			// Only apps that actually have a domain need the proxy on their network.
			networks[naming.ProjectNetworkName(row.ProjectSlug)] = struct{}{}
		}
	}

	names := make([]string, 0, len(networks))
	for n := range networks {
		names = append(names, n)
	}
	return configs, names, nil
}

// ensureNetworks re-joins the proxy container to every project network it needs.
//
// The deploy worker does this when an app is deployed, and that was the only
// place it happened — so a proxy container that got *recreated* (any compose up,
// an upgrade) came back with none of them, and every app domain answered 502
// until each app was redeployed. Docker treats an existing attachment as a
// no-op, so this is cheap to re-assert on every pass.
func (r *Reconciler) ensureNetworks(ctx context.Context, networks []string) {
	if r.net == nil || r.proxyContainer == "" {
		return
	}
	for _, network := range networks {
		if err := r.net.ConnectContainerToNetwork(ctx, r.proxyContainer, network); err != nil {
			// The proxy may legitimately live outside Docker, so this is a warning,
			// not a failed pass.
			slog.Warn("proxy reconciler: failed to attach proxy to project network",
				"proxy", r.proxyContainer, "network", network, "error", err)
		}
	}
}

// syncCertificates pushes the PEM pair of every ssl_mode=custom domain into the
// proxy's in-band certificate store — plus the dashboard's own, which has no
// domains row and so would otherwise be left with nothing to serve.
func (r *Reconciler) syncCertificates(ctx context.Context, dashboardHost, dashboardMode, dashboardCertID string) error {
	rows, err := r.queries.ListCustomCertDomains(ctx)
	if err != nil {
		return err
	}

	certs := make([]HostCertificate, 0, len(rows)+1)
	for _, row := range rows {
		certPEM, err := r.dec.Decrypt(row.CertPemEncrypted)
		if err != nil {
			slog.Warn("proxy reconciler: failed to decrypt certificate", "hostname", row.Hostname, "error", err)
			continue
		}
		keyPEM, err := r.dec.Decrypt(row.KeyPemEncrypted)
		if err != nil {
			slog.Warn("proxy reconciler: failed to decrypt certificate key", "hostname", row.Hostname, "error", err)
			continue
		}
		certs = append(certs, HostCertificate{
			Hostname: row.Hostname,
			CertPEM:  string(certPEM),
			KeyPEM:   string(keyPEM),
		})
	}

	if dashboardHost != "" && dashboardMode == SSLModeCustom && dashboardCertID != "" {
		if cert, ok := r.dashboardCertificate(ctx, dashboardHost, dashboardCertID); ok {
			certs = append(certs, cert)
		}
	}

	return r.proxy.SyncCertificates(ctx, certs)
}

// dashboardCertificate loads and decrypts the certificate the dashboard is
// configured to serve. A missing or undecryptable certificate is a warning, not
// a failure: the rest of the certificates must still reach Caddy.
func (r *Reconciler) dashboardCertificate(ctx context.Context, hostname, certID string) (HostCertificate, bool) {
	var id pgtype.UUID
	if err := id.Scan(certID); err != nil {
		slog.Warn("proxy reconciler: dashboard certificate id is not a uuid", "id", certID)
		return HostCertificate{}, false
	}
	row, err := r.queries.GetCertificate(ctx, id)
	if err != nil {
		slog.Warn("proxy reconciler: dashboard certificate not found", "id", certID, "error", err)
		return HostCertificate{}, false
	}
	certPEM, err := r.dec.Decrypt(row.CertPemEncrypted)
	if err != nil {
		slog.Warn("proxy reconciler: failed to decrypt dashboard certificate", "error", err)
		return HostCertificate{}, false
	}
	keyPEM, err := r.dec.Decrypt(row.KeyPemEncrypted)
	if err != nil {
		slog.Warn("proxy reconciler: failed to decrypt dashboard certificate key", "error", err)
		return HostCertificate{}, false
	}
	return HostCertificate{
		Hostname: hostname,
		CertPEM:  string(certPEM),
		KeyPEM:   string(keyPEM),
	}, true
}

// dashboardTLS reads how the operator wants the dashboard served. An unset or
// unreadable setting means "no dashboard route" rather than an error: the
// catch-all still serves the dashboard on any host over plain HTTP.
//
// An absent mode is automatic, which is what every install did before the mode
// existed — so an upgrade keeps the certificate it already has.
func (r *Reconciler) dashboardTLS(ctx context.Context) (hostname, sslMode, certID string) {
	setting, err := r.queries.GetSetting(ctx, SettingDashboardDomain)
	if err != nil {
		return "", "", ""
	}
	hostname = strings.TrimSpace(setting.Value)
	if hostname == "" {
		return "", "", ""
	}

	sslMode = SSLModeAutomatic
	if s, err := r.queries.GetSetting(ctx, SettingDashboardSSLMode); err == nil {
		if m := strings.TrimSpace(s.Value); ValidSSLMode(m) && m != "" {
			sslMode = m
		}
	}
	if s, err := r.queries.GetSetting(ctx, SettingDashboardCertificateID); err == nil {
		certID = strings.TrimSpace(s.Value)
	}
	return hostname, sslMode, certID
}

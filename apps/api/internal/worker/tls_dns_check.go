package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

// DNS precheck.
//
// By far the most common reason a certificate never appears is that the
// hostname does not point at this server: the ACME challenge is answered by
// whatever *is* at that address, so issuance quietly fails forever. Caddy will
// keep retrying and the user is left staring at a domain that never goes green.
//
// Neither Coolify nor Dokploy surfaces this, which is why it is worth doing the
// lookup ourselves and saying so in plain words.

// dnsCheckTimeout bounds the resolver call: the probe sweep runs every minute
// over every domain, and a black-holed resolver must not stall it.
const dnsCheckTimeout = 3 * time.Second

// checkDNS reports a human-readable mismatch when hostname resolves somewhere
// other than this server, or "" when it matches, cannot be determined, or the
// server's own public IP is unknown. It is deliberately conservative: a false
// "your DNS is wrong" is worse than staying quiet.
func (h *TaskHandler) checkDNS(ctx context.Context, hostname string) string {
	public := h.publicIP()
	if public == "" {
		return "" // no baseline to compare against — skip the check
	}

	ctx, cancel := context.WithTimeout(ctx, dnsCheckTimeout)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		// NXDOMAIN is worth reporting — the record simply is not there — but a
		// timeout or a broken resolver says nothing about the user's DNS.
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return fmt.Sprintf("%s does not resolve — add a DNS A record pointing at %s, or a certificate can never be issued", hostname, public)
		}
		slog.Debug("tls dns check: lookup failed", "hostname", hostname, "error", err)
		return ""
	}

	for _, a := range addrs {
		if a == public {
			return ""
		}
	}

	// Resolving somewhere else is not always fatal — a CDN or proxy in front of
	// us (Cloudflare's orange cloud) is a legitimate setup — so the wording says
	// what we see and what it implies, rather than asserting a misconfiguration.
	return fmt.Sprintf(
		"%s resolves to %s, not this server (%s). Unless a proxy sits in front, certificate issuance will fail.",
		hostname, strings.Join(addrs, ", "), public,
	)
}

// publicIPOnce caches autodetection: it opens a UDP socket, which is cheap but
// pointless to redo for every domain on every sweep.
var (
	publicIPOnce  sync.Once
	publicIPValue string
)

// publicIP returns the configured public IP, falling back to the local address
// the kernel would use for outbound traffic. That fallback is right for a plain
// VPS and wrong behind NAT, which is exactly why BELUNE_PUBLIC_IP exists.
func (h *TaskHandler) publicIP() string {
	if h.Config != nil && h.Config.PublicIP != "" {
		return h.Config.PublicIP
	}
	publicIPOnce.Do(func() {
		// No packets are sent: connecting a UDP socket only fixes the route, and
		// the local address it picks is the one we would egress from.
		conn, err := net.Dial("udp", "1.1.1.1:80")
		if err != nil {
			slog.Debug("tls dns check: could not autodetect public IP", "error", err)
			return
		}
		defer conn.Close()
		addr, ok := conn.LocalAddr().(*net.UDPAddr)
		if !ok {
			return
		}
		// A private or loopback address means we are behind NAT or on a dev box.
		// Comparing a public DNS record against it would flag every domain as
		// misconfigured, so stay quiet and let the operator set BELUNE_PUBLIC_IP.
		if !isPublicIP(addr.IP) {
			slog.Debug("tls dns check: autodetected a non-public address; set BELUNE_PUBLIC_IP to enable the DNS precheck",
				"detected", addr.IP.String())
			return
		}
		publicIPValue = addr.IP.String()
	})
	return publicIPValue
}

// isPublicIP reports whether ip is globally routable — i.e. something a user's
// DNS record could legitimately point at.
func isPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}

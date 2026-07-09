package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/naming"
	"github.com/weiling79/belune/internal/runtime"
)

// Read-only Docker inspect endpoints powering the admin Docker page. These
// surface every resource on the host (not just platform-managed ones) for
// observability. They are strictly read-only — no lifecycle or destructive
// operations are exposed here; managed mutation stays in the app/database
// handlers that keep Postgres and Caddy in sync.

// Label keys the platform stamps on the resources it creates. Mirrored here
// (the docker package keeps them unexported) so the API can flag ownership.
const (
	dockerLabelManagedBy    = "managed-by"
	dockerLabelManagedValue = "belune"
	dockerLabelAppID        = "application-id"
	dockerLabelDatabaseID   = "database-id"
	dockerLabelCache        = "belune-cache"
	dockerLabelData         = "belune-data"
)

// dockerTimeout bounds each Docker probe so one hung daemon call can't stall
// the request (system df in particular scans all volumes/images).
const dockerTimeout = 8 * time.Second

func isManaged(labels map[string]string) bool {
	return labels[dockerLabelManagedBy] == dockerLabelManagedValue
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// dockerOwner links a resource back to the platform object that owns it.
type dockerOwner struct {
	Type      string `json:"type"` // "application" | "database"
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id"`
}

type dockerCounts struct {
	ContainersRunning int `json:"containers_running"`
	ContainersTotal   int `json:"containers_total"`
	Images            int `json:"images"`
	Volumes           int `json:"volumes"`
}

type dockerOverviewResponse struct {
	Info      *runtime.DockerSystemInfo `json:"info"`
	DiskUsage *runtime.DockerDiskUsage  `json:"disk_usage"`
	Counts    dockerCounts              `json:"counts"`
}

// GetDockerOverview returns `docker info` + `docker system df` for the admin
// Docker overview tab. GET /api/docker/overview (admin only).
func (h *Handler) GetDockerOverview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), dockerTimeout)
	defer cancel()

	info, err := h.runtime.SystemInfo(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to read Docker info")
		return
	}
	du, err := h.runtime.SystemDiskUsage(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to read Docker disk usage")
		return
	}

	writeJSON(w, http.StatusOK, dockerOverviewResponse{
		Info:      info,
		DiskUsage: du,
		Counts: dockerCounts{
			ContainersRunning: info.ContainersRunning,
			ContainersTotal:   info.Containers,
			Images:            info.Images,
			Volumes:           du.Volumes.Count,
		},
	})
}

type dockerContainerResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Status    string            `json:"status"`
	Ports     map[string]string `json:"ports"`
	Managed   bool              `json:"managed"`
	Owner     *dockerOwner      `json:"owner,omitempty"`
	CreatedAt string            `json:"created_at"`
}

// ListDockerContainers lists every container on the host (running + stopped),
// flagging platform-managed ones and linking them to their app/database.
// GET /api/docker/containers (admin only).
func (h *Handler) ListDockerContainers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), dockerTimeout)
	defer cancel()

	containers, err := h.runtime.ListAllContainers(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to list containers")
		return
	}

	owners := h.resolveOwners(ctx, containers)
	out := make([]dockerContainerResponse, 0, len(containers))
	for _, c := range containers {
		out = append(out, dockerContainerResponse{
			ID:        c.ID,
			Name:      c.Name,
			Image:     c.Image,
			Status:    c.Status,
			Ports:     c.Ports,
			Managed:   isManaged(c.Labels),
			Owner:     owners[c.ID],
			CreatedAt: rfc3339(c.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// resolveOwners best-effort resolves the app/database each managed container
// belongs to (via its labels) so the UI can deep-link. Distinct IDs are
// resolved once; lookup failures are ignored (the row simply has no owner).
func (h *Handler) resolveOwners(ctx context.Context, containers []runtime.ContainerInfo) map[string]*dockerOwner {
	owners := make(map[string]*dockerOwner, len(containers))
	appCache := make(map[string]*dockerOwner)
	dbCache := make(map[string]*dockerOwner)

	for _, c := range containers {
		if appID := c.Labels[dockerLabelAppID]; appID != "" {
			owner, seen := appCache[appID]
			if !seen {
				owner = h.lookupAppOwner(ctx, appID)
				appCache[appID] = owner
			}
			if owner != nil {
				owners[c.ID] = owner
			}
			continue
		}
		if dbID := c.Labels[dockerLabelDatabaseID]; dbID != "" {
			owner, seen := dbCache[dbID]
			if !seen {
				owner = h.lookupDatabaseOwner(ctx, dbID)
				dbCache[dbID] = owner
			}
			if owner != nil {
				owners[c.ID] = owner
			}
		}
	}
	return owners
}

func (h *Handler) lookupAppOwner(ctx context.Context, id string) *dockerOwner {
	var uid pgtype.UUID
	if err := uid.Scan(id); err != nil {
		return nil
	}
	app, err := h.queries.GetApplication(ctx, uid)
	if err != nil {
		return nil
	}
	return &dockerOwner{
		Type:      "application",
		ID:        uuidToString(app.ID),
		Name:      app.Name,
		ProjectID: uuidToString(app.ProjectID),
	}
}

func (h *Handler) lookupDatabaseOwner(ctx context.Context, id string) *dockerOwner {
	var uid pgtype.UUID
	if err := uid.Scan(id); err != nil {
		return nil
	}
	db, err := h.queries.GetDatabase(ctx, uid)
	if err != nil {
		return nil
	}
	return &dockerOwner{
		Type:      "database",
		ID:        uuidToString(db.ID),
		Name:      db.Name,
		ProjectID: uuidToString(db.ProjectID),
	}
}

type dockerImageResponse struct {
	ID         string       `json:"id"`
	RepoTags   []string     `json:"repo_tags"`
	Size       int64        `json:"size"`
	SharedSize int64        `json:"shared_size"`
	Containers int64        `json:"containers"`
	Dangling   bool         `json:"dangling"`
	Managed    bool         `json:"managed"`
	Owner      *dockerOwner `json:"owner,omitempty"`
	CreatedAt  string       `json:"created_at"`
}

// ListDockerImages lists all images on the host. GET /api/docker/images (admin only).
func (h *Handler) ListDockerImages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), dockerTimeout)
	defer cancel()

	images, err := h.runtime.ListImages(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to list images")
		return
	}

	// Attribute images to the app that built them. Docker image labels are only
	// reliable on the Dockerfile builder, so ownership is resolved from the
	// deployments table (image_tag → application) instead — uniform across all
	// builders (Dockerfile / Railpack / buildpacks / prebuilt).
	owners := h.imageTagOwners(ctx)

	out := make([]dockerImageResponse, 0, len(images))
	for _, img := range images {
		var owner *dockerOwner
		for _, tag := range img.RepoTags {
			if o, ok := owners[tag]; ok {
				owner = o
				break
			}
		}
		out = append(out, dockerImageResponse{
			ID:         img.ID,
			RepoTags:   img.RepoTags,
			Size:       img.Size,
			SharedSize: img.SharedSize,
			Containers: img.Containers,
			Dangling:   img.Dangling,
			Managed:    isManaged(img.Labels),
			Owner:      owner,
			CreatedAt:  rfc3339(img.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// imageTagOwners maps each built image tag to the app that produced it, from the
// deployments table. Failures degrade gracefully to an empty map (images then
// simply show no owner). Keyed by the exact `repo:tag` string Docker reports.
func (h *Handler) imageTagOwners(ctx context.Context) map[string]*dockerOwner {
	rows, err := h.queries.ListImageTagOwners(ctx)
	if err != nil {
		return map[string]*dockerOwner{}
	}
	m := make(map[string]*dockerOwner, len(rows))
	for _, row := range rows {
		if !row.ImageTag.Valid {
			continue
		}
		m[row.ImageTag.String] = &dockerOwner{
			Type:      "application",
			ID:        uuidToString(row.ApplicationID),
			Name:      row.ApplicationName,
			ProjectID: uuidToString(row.ProjectID),
		}
	}
	return m
}

type dockerVolumeResponse struct {
	Name       string       `json:"name"`
	Driver     string       `json:"driver"`
	Mountpoint string       `json:"mountpoint"`
	Scope      string       `json:"scope"`
	Size       int64        `json:"size"`
	RefCount   int64        `json:"ref_count"`
	Managed    bool         `json:"managed"`
	Kind       string       `json:"kind"` // "data" | "cache" | "" (platform volume classification)
	Owner      *dockerOwner `json:"owner,omitempty"`
	CreatedAt  string       `json:"created_at"`
}

// ListDockerVolumes lists all volumes on the host with sizes.
// GET /api/docker/volumes (admin only).
func (h *Handler) ListDockerVolumes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), dockerTimeout)
	defer cancel()

	volumes, err := h.runtime.ListVolumes(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to list volumes")
		return
	}

	// Volumes carry no application-id label, so ownership is resolved by
	// reconstructing the platform's deterministic volume names from the
	// application_volumes / databases tables (see volumeOwners).
	owners := h.volumeOwners(ctx)

	out := make([]dockerVolumeResponse, 0, len(volumes))
	for _, v := range volumes {
		out = append(out, dockerVolumeResponse{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Scope:      v.Scope,
			Size:       v.Size,
			RefCount:   v.RefCount,
			Managed:    isManaged(v.Labels),
			Kind:       volumeKind(v.Labels),
			Owner:      owners[v.Name],
			CreatedAt:  rfc3339(v.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// volumeOwners maps each platform-created Docker volume name to the app or
// database that owns it. Volumes have no application-id label, but their names
// are deterministic — app volumes are naming.AppVolumeName(appID, name) and
// managed-DB volumes are "<dbSlug>-vol" — so ownership is reconstructed from
// the DB. Failures degrade to an empty map (volumes then show no owner).
func (h *Handler) volumeOwners(ctx context.Context) map[string]*dockerOwner {
	owners := map[string]*dockerOwner{}

	if appVols, err := h.queries.ListApplicationVolumeOwners(ctx); err == nil {
		for _, v := range appVols {
			name := naming.AppVolumeName(uuidToString(v.ApplicationID), v.Name)
			owners[name] = &dockerOwner{
				Type:      "application",
				ID:        uuidToString(v.ApplicationID),
				Name:      v.ApplicationName,
				ProjectID: uuidToString(v.ProjectID),
			}
		}
	}

	if dbs, err := h.queries.ListAllDatabases(ctx); err == nil {
		for _, db := range dbs {
			owners[db.Slug+"-vol"] = &dockerOwner{
				Type:      "database",
				ID:        uuidToString(db.ID),
				Name:      db.Name,
				ProjectID: uuidToString(db.ProjectID),
			}
		}
	}

	return owners
}

// volumeKind classifies a platform volume as persistent data or disposable
// cache from its labels, so the UI can warn that data volumes are never pruned.
func volumeKind(labels map[string]string) string {
	if labels[dockerLabelData] == "true" {
		return "data"
	}
	if labels[dockerLabelCache] == "true" {
		return "cache"
	}
	return ""
}

type dockerNetworkContainerResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IPv4Address string `json:"ipv4_address"`
}

type dockerNetworkResponse struct {
	ID         string                           `json:"id"`
	Name       string                           `json:"name"`
	Driver     string                           `json:"driver"`
	Scope      string                           `json:"scope"`
	Internal   bool                             `json:"internal"`
	Managed    bool                             `json:"managed"`
	Containers []dockerNetworkContainerResponse `json:"containers"`
	CreatedAt  string                           `json:"created_at"`
}

// ListDockerNetworks lists all networks on the host with attached containers.
// GET /api/docker/networks (admin only).
func (h *Handler) ListDockerNetworks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), dockerTimeout)
	defer cancel()

	networks, err := h.runtime.ListNetworks(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to list networks")
		return
	}
	out := make([]dockerNetworkResponse, 0, len(networks))
	for _, n := range networks {
		attached := make([]dockerNetworkContainerResponse, 0, len(n.Containers))
		for _, c := range n.Containers {
			attached = append(attached, dockerNetworkContainerResponse{
				ID:          c.ID,
				Name:        c.Name,
				IPv4Address: c.IPv4Address,
			})
		}
		out = append(out, dockerNetworkResponse{
			ID:         n.ID,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			Internal:   n.Internal,
			Managed:    isManaged(n.Labels),
			Containers: attached,
			CreatedAt:  rfc3339(n.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

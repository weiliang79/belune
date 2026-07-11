package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/naming"
	"github.com/weiliang79/belune/internal/store/generated"
)

// safeMountPathChars restricts mount paths to a conservative character set.
// Beyond hygiene, this keeps the value safe when it is later interpolated into
// shell-driven volume operations (e.g. the backup/restore helper): no spaces,
// quotes, or shell metacharacters can appear.
var safeMountPathChars = regexp.MustCompile(`^/[A-Za-z0-9_./-]+$`)

// reservedMountPaths are in-container paths a persistent volume must never
// shadow: "/" would hide the whole rootfs, and the others are either managed
// by the container hardening (tmpfs at /tmp and /run) or kernel/pseudo
// filesystems that must not be masked by a data volume.
var reservedMountPaths = map[string]bool{
	"/":     true,
	"/proc": true,
	"/sys":  true,
	"/dev":  true,
	"/tmp":  true,
	"/run":  true,
	"/etc":  true,
	"/boot": true,
	"/bin":  true,
	"/sbin": true,
	"/lib":  true,
	"/usr":  true,
}

// reservedMountPrefixes reject any mount nested under a kernel/pseudo
// filesystem, e.g. "/proc/foo".
var reservedMountPrefixes = []string{"/proc/", "/sys/", "/dev/"}

// validateMountPath enforces that a volume mount path is an absolute, clean,
// non-reserved in-container path. It is deliberately package-level and pure so
// it can be unit-tested without a running handler.
func validateMountPath(p string) error {
	if p == "" {
		return errors.New("mount_path is required")
	}
	if !strings.HasPrefix(p, "/") {
		return errors.New("mount_path must be an absolute path (start with /)")
	}
	if path.Clean(p) != p {
		return errors.New("mount_path must be a clean path (no '.', '..', '//', or trailing slash)")
	}
	if !safeMountPathChars.MatchString(p) {
		return errors.New("mount_path may only contain letters, numbers, '_', '.', '-', and '/'")
	}
	if reservedMountPaths[p] {
		return fmt.Errorf("mount_path %q is reserved", p)
	}
	for _, prefix := range reservedMountPrefixes {
		if strings.HasPrefix(p, prefix) {
			return fmt.Errorf("mount_path %q is under a reserved path", p)
		}
	}
	if len(p) > 500 {
		return errors.New("mount_path is too long")
	}
	return nil
}

// applicationVolumeResponse is the API shape for a persistent volume, enriched
// with the live on-disk size of the backing Docker volume (0 until the volume
// has been materialised on the first deploy).
type applicationVolumeResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

func (h *Handler) ListApplicationVolumes(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, applicationUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	volumes, err := h.queries.ListApplicationVolumes(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list volumes")
		return
	}

	// Best-effort live sizes: one DiskUsage call for all of this app's volumes.
	// A runtime error here must not fail the listing — sizes fall back to 0.
	names := make([]string, 0, len(volumes))
	for _, v := range volumes {
		names = append(names, naming.AppVolumeName(applicationID, v.Name))
	}
	sizes, err := h.runtime.VolumeSizes(r.Context(), names)
	if err != nil {
		slog.Debug("list volumes: failed to fetch sizes", "application_id", applicationID, "error", err)
		sizes = nil
	}

	out := make([]applicationVolumeResponse, 0, len(volumes))
	for _, v := range volumes {
		out = append(out, applicationVolumeResponse{
			ID:        uuidToString(v.ID),
			Name:      v.Name,
			MountPath: v.MountPath,
			SizeBytes: sizes[naming.AppVolumeName(applicationID, v.Name)],
			CreatedAt: v.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	writeJSON(w, http.StatusOK, out)
}

type createVolumeRequest struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

func (h *Handler) CreateApplicationVolume(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, applicationUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req createVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// name is stored already-slugified and doubles as the Docker volume-name
	// component, so it must be non-empty and unique per application.
	nameSlug := naming.Slugify(req.Name)
	if nameSlug == "" {
		writeError(w, http.StatusBadRequest, "name is required and must contain alphanumeric characters")
		return
	}
	if len(nameSlug) > 40 {
		writeError(w, http.StatusBadRequest, "name is too long")
		return
	}
	if err := validateMountPath(req.MountPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	vol, err := h.queries.CreateApplicationVolume(r.Context(), generated.CreateApplicationVolumeParams{
		ApplicationID: applicationUUID,
		Name:          nameSlug,
		MountPath:     req.MountPath,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "a volume with this name or mount path already exists on this application")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create volume")
		return
	}

	h.audit(r, "create_volume", "application_volume", uuidToString(vol.ID), map[string]any{
		"application_id": applicationID,
		"name":           nameSlug,
		"mount_path":     req.MountPath,
	})

	writeJSON(w, http.StatusCreated, applicationVolumeResponse{
		ID:        uuidToString(vol.ID),
		Name:      vol.Name,
		MountPath: vol.MountPath,
		SizeBytes: 0,
		CreatedAt: vol.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *Handler) DeleteApplicationVolume(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	volumeID := chi.URLParam(r, "volumeId")
	var volumeUUID pgtype.UUID
	if err := volumeUUID.Scan(volumeID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid volume id")
		return
	}

	if !h.canAccessApplication(r, applicationUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	vol, err := h.queries.GetApplicationVolume(r.Context(), volumeUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "volume not found")
		return
	}
	// Defence in depth: the volume must belong to the application in the path.
	if vol.ApplicationID != applicationUUID {
		writeError(w, http.StatusNotFound, "volume not found")
		return
	}

	if err := h.queries.DeleteApplicationVolume(r.Context(), volumeUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete volume")
		return
	}

	// With delete_data=true the caller wants the underlying data destroyed too.
	// Best-effort: the Docker volume may not exist yet (never deployed) or may
	// still be in use by a running container (removal blocked until the app is
	// redeployed without it). Neither case should fail the row deletion, which
	// is what detaches the mount on the next deploy.
	deleteData := r.URL.Query().Get("delete_data") == "true"
	warning := ""
	if deleteData {
		volName := naming.AppVolumeName(applicationID, vol.Name)
		if err := h.runtime.RemoveVolume(r.Context(), volName); err != nil {
			slog.Warn("delete volume: failed to remove docker volume", "volume", volName, "error", err)
			warning = "volume record deleted, but the underlying data volume could not be removed yet (it may be in use — redeploy the application, then remove it)"
		}
	}

	h.audit(r, "delete_volume", "application_volume", volumeID, map[string]any{
		"application_id": applicationID,
		"name":           vol.Name,
		"delete_data":    deleteData,
	})

	resp := map[string]string{"status": "deleted"}
	if warning != "" {
		resp["warning"] = warning
	}
	writeJSON(w, http.StatusOK, resp)
}

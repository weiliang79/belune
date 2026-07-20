package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/store/generated"
)

// The health check has two mechanisms behind one type field, so it is written
// through its own endpoint rather than the general update: the fields that
// belong to the unselected mechanism must be cleared as a unit, or a later
// deploy would apply a stale command it can no longer see in the UI.
//
//	http    — control-plane probe of an HTTP path after deploy (existing)
//	command — native Docker HEALTHCHECK, run in the container, continuous, and
//	          feeding the application's status via the eventwatcher
//	none    — no check
type healthCheckRequest struct {
	Type string `json:"type"`

	// http
	Path         string `json:"path"`
	ExpectStatus int    `json:"expect_status"`

	// command
	Command            string `json:"command"`
	IntervalSeconds    int    `json:"interval_seconds"`
	RetriesCount       int    `json:"retries"`
	StartPeriodSeconds int    `json:"start_period_seconds"`

	// shared
	TimeoutSeconds int `json:"timeout_seconds"`
}

func nullInt4(v int) pgtype.Int4 {
	if v <= 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func nullText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

// validateHealthCheck rejects a request whose fields do not match its type, and
// returns the params with the unselected mechanism's fields nulled so the
// stored row is coherent.
func validateHealthCheck(id pgtype.UUID, req healthCheckRequest) (generated.SetApplicationHealthCheckParams, error) {
	p := generated.SetApplicationHealthCheckParams{ID: id, HealthCheckType: req.Type}

	switch req.Type {
	case "none":
		// Everything stays null.
		return p, nil

	case "http":
		if req.Path == "" {
			return p, errors.New("an HTTP health check needs a path")
		}
		if req.Path[0] != '/' {
			return p, errors.New("the health check path must start with /")
		}
		if req.ExpectStatus != 0 && (req.ExpectStatus < 100 || req.ExpectStatus > 599) {
			return p, errors.New("expected status must be a valid HTTP status code")
		}
		p.HealthCheckPath = nullText(req.Path)
		p.HealthCheckExpectStatus = nullInt4(req.ExpectStatus)
		p.HealthCheckTimeoutSeconds = nullInt4(req.TimeoutSeconds)
		return p, nil

	case "command":
		if req.Command == "" {
			return p, errors.New("a command health check needs a command")
		}
		p.HealthCheckCommand = nullText(req.Command)
		p.HealthCheckIntervalSeconds = nullInt4(req.IntervalSeconds)
		p.HealthCheckRetries = nullInt4(req.RetriesCount)
		p.HealthCheckStartPeriodSeconds = nullInt4(req.StartPeriodSeconds)
		p.HealthCheckTimeoutSeconds = nullInt4(req.TimeoutSeconds)
		return p, nil

	default:
		return p, errors.New("type must be one of: none, http, command")
	}
}

// SetHealthCheck configures how an application's health is checked. A change
// takes effect on the next deploy — the check is baked into the container at
// create time (command) or run by the deploy (http) — so the pending-change
// marker is stamped, and the change is applied when the user next deploys.
func (h *Handler) SetHealthCheck(w http.ResponseWriter, r *http.Request) {
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

	var req healthCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params, err := validateHealthCheck(applicationUUID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	app, err := h.queries.SetApplicationHealthCheck(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update health check")
		return
	}

	// A command check is baked into the container image config, and the HTTP
	// probe runs during deploy — either way the running container does not
	// reflect the change until the next deploy.
	h.markConfigChanged(r.Context(), applicationUUID)

	h.audit(r, "set_health_check", "application", applicationID, map[string]any{"type": req.Type})

	writeJSON(w, http.StatusOK, toApplicationResponse(app))
}

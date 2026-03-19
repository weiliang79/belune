package handler

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/sse"
)

func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)

	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	follow := r.URL.Query().Get("follow") == "true"

	logs, err := h.runtime.ContainerLogs(r.Context(), containerName, follow)
	if err != nil {
		writer.SendEvent("error", fmt.Sprintf("failed to get logs: %v", err))
		return
	}
	defer logs.Close()

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()
		if err := writer.SendData(line); err != nil {
			return // Client disconnected
		}
	}

	writer.SendEvent("done", "log stream ended")
}

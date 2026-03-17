package handler

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/sse"
)

func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	containerName := fmt.Sprintf("paas-%s", serviceID[:8])

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

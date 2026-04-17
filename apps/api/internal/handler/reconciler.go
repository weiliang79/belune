package handler

import (
	"net/http"

	"github.com/ungweiliang/selfhost-paas/internal/proxy"
)

// GetProxyReconcilerStatus returns the most recent reconciliation pass state.
// Useful for operators tracking whether Caddy drifts from the DB-declared
// routes between deploys.
// GET /api/proxy/reconciler (admin only)
func (h *Handler) GetProxyReconcilerStatus(w http.ResponseWriter, r *http.Request) {
	if h.reconciler == nil {
		writeJSON(w, http.StatusOK, proxy.ReconcilerStatus{})
		return
	}
	writeJSON(w, http.StatusOK, h.reconciler.Status())
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	result, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	// Set HTTP-only cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(24 * time.Hour / time.Second),
	})

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Blacklist the current token so it can't be reused
	tokenString := extractTokenFromRequest(r)
	if tokenString != "" {
		_ = h.auth.BlacklistToken(r.Context(), tokenString)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// extractTokenFromRequest extracts the JWT from Authorization header or cookie.
func extractTokenFromRequest(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	if cookie, err := r.Cookie("token"); err == nil {
		return cookie.Value
	}
	return ""
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         userID,
		"email":      user.Email,
		"role":       user.Role,
		"username":   user.Username,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
	})
}

type updateProfileRequest struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.queries.UpdateUserProfile(r.Context(), generated.UpdateUserProfileParams{
		ID:        userUUID,
		Username:  req.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) ChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	// Get current user from DB to verify current password
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := h.queries.UpdateUserPassword(r.Context(), generated.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: string(hash),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

type setupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	required, err := h.auth.IsSetupRequired(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check setup status")
		return
	}

	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]bool{"setup_required": required})
		return
	}

	// POST — create first admin user
	if !required {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	user, err := h.auth.Register(r.Context(), req.Email, req.Password, "admin", req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"role":     user.Role,
		"username": user.Username,
	})
}

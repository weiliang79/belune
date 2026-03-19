package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	writeJSON(w, http.StatusOK, users)
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
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

	if req.Role != "admin" && req.Role != "member" {
		writeError(w, http.StatusBadRequest, "role must be admin or member")
		return
	}

	user, err := h.auth.Register(r.Context(), req.Email, req.Password, req.Role)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			writeError(w, http.StatusConflict, "user already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, generated.ListUsersRow{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	})
}

type updateUserRoleRequest struct {
	Role string `json:"role"`
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req updateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != "admin" && req.Role != "member" {
		writeError(w, http.StatusBadRequest, "role must be admin or member")
		return
	}

	// Prevent leaving zero admins: if demoting an admin, check count
	target, err := h.queries.GetUserByID(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if target.Role == "admin" && req.Role != "admin" {
		count, err := h.queries.CountAdmins(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check admin count")
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "cannot demote the last admin")
			return
		}
	}

	updated, err := h.queries.UpdateUserRole(r.Context(), generated.UpdateUserRoleParams{
		ID:   uuid,
		Role: req.Role,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user role")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Prevent self-deletion
	callerID := middleware.UserIDFromContext(r.Context())
	if id == callerID {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	// Prevent deleting last admin
	target, err := h.queries.GetUserByID(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if target.Role == "admin" {
		count, err := h.queries.CountAdmins(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check admin count")
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "cannot delete the last admin")
			return
		}
	}

	if err := h.queries.DeleteUser(r.Context(), uuid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type resetUserPasswordRequest struct {
	Password string `json:"password"`
}

func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req resetUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := h.queries.UpdateUserPassword(r.Context(), generated.UpdateUserPasswordParams{
		ID:           uuid,
		PasswordHash: string(hash),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

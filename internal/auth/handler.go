package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"nginx-proxy-manager/internal/handler"
)

type User struct {
	ID        int64
	Username  string
	CreatedAt string
}

type Store interface {
	Authenticate(username, password string) (int64, error)
	RequiresPasswordChange(userID int64) (bool, error)
	ChangePassword(userID int64, oldPassword, newPassword string) error
	CreateSession(userID int64) (string, error)
	ValidateSession(sessionID string) (int64, error)
	DeleteSession(sessionID string) error
	CreateUser(username, password string) error
	DeleteUser(userID int64) error
	ListUsers() ([]User, error)
}

type contextKey string

const UserIDKey contextKey = "userID"

func UserIDFromCtx(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(UserIDKey).(int64)
	return id, ok
}

type Handler struct {
	store  Store
	render *handler.Render
}

func NewHandler(store Store, render *handler.Render) *Handler {
	return &Handler{store: store, render: render}
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render.Login(w, nil)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	userID, err := h.store.Authenticate(username, password)
	if err != nil {
		h.render.Login(w, map[string]string{"Error": "Invalid credentials"})
		return
	}

	sessionID, err := h.store.CreateSession(userID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	requiresChange, err := h.store.RequiresPasswordChange(userID)
	if err == nil && requiresChange {
		http.Redirect(w, r, "/change-password", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		_ = h.store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: "session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) ChangePasswordPage(w http.ResponseWriter, r *http.Request) {
	h.render.ChangePassword(w, nil)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	userID, ok := UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	oldPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if newPassword != confirmPassword {
		h.render.ChangePassword(w, map[string]string{"Error": "New passwords do not match"})
		return
	}
	if len(newPassword) < 6 {
		h.render.ChangePassword(w, map[string]string{"Error": "Password must be at least 6 characters"})
		return
	}

	if err := h.store.ChangePassword(userID, oldPassword, newPassword); err != nil {
		h.render.ChangePassword(w, map[string]string{"Error": err.Error()})
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) UsersPage(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	h.render.Users(w, map[string]interface{}{
		"Users": users,
	})
}

func (h *Handler) renderUsersWithError(w http.ResponseWriter, msg string) {
	users, err := h.store.ListUsers()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	h.render.Users(w, map[string]interface{}{
		"Users": users,
		"Error": msg,
	})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderUsersWithError(w, "Bad request")
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" {
		h.renderUsersWithError(w, "Username is required")
		return
	}
	if len(password) < 6 {
		h.renderUsersWithError(w, "Password must be at least 6 characters")
		return
	}

	if err := h.store.CreateUser(username, password); err != nil {
		h.renderUsersWithError(w, err.Error())
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	targetID := r.PathValue("id")
	if targetID == "" {
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	var id int64
	if _, err := fmt.Sscanf(targetID, "%d", &id); err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if id == currentUserID {
		http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteUser(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

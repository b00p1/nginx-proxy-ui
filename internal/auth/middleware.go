package auth

import (
	"context"
	"net/http"
)

type Middleware struct {
	store Store
}

func NewMiddleware(store Store) *Middleware {
	return &Middleware{store: store}
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		userID, err := m.store.ValidateSession(cookie.Value)
		if err != nil {
			http.SetCookie(w, &http.Cookie{
				Name: "session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
			})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if r.URL.Path != "/change-password" {
			requiresChange, err := m.store.RequiresPasswordChange(userID)
			if err == nil && requiresChange {
				http.Redirect(w, r, "/change-password", http.StatusSeeOther)
				return
			}
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

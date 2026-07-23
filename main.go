package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"nginx-proxy-manager/internal/auth"
	"nginx-proxy-manager/internal/config"
	"nginx-proxy-manager/internal/db"
	"nginx-proxy-manager/internal/handler"
	"nginx-proxy-manager/internal/nginx"
)

//go:embed web/templates
var templatesRoot embed.FS

func main() {
	streamPath := os.Getenv("STREAM_CONF")
	if streamPath == "" {
		streamPath = "/etc/nginx/stream-manager/stream.conf"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/etc/nginx-proxy-manager/auth.db"
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8742"
	}

	store, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Close()

	tmplFS, err := fs.Sub(templatesRoot, "web/templates")
	if err != nil {
		log.Fatalf("templates fs: %v", err)
	}
	rend, err := handler.NewRender(tmplFS)
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	mgr := config.NewManager(streamPath)
	ctl := nginx.New()

	authHandler := auth.NewHandler(store, rend)
	authMiddleware := auth.NewMiddleware(store)
	apiHandler := handler.New(rend, mgr, ctl)

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /login", authHandler.LoginPage)
	mux.HandleFunc("POST /login", authHandler.Login)

	// Authenticated routes
	mux.Handle("GET /", authMiddleware.RequireAuth(http.HandlerFunc(apiHandler.Dashboard)))
	mux.Handle("POST /logout", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /change-password", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.ChangePasswordPage)))
	mux.Handle("POST /change-password", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.ChangePassword)))
	mux.Handle("GET /users", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.UsersPage)))
	mux.Handle("POST /users", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.CreateUser)))
	mux.Handle("POST /users/{id}/delete", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.DeleteUser)))

	// API routes
	mux.Handle("GET /api/proxies", authMiddleware.RequireAuth(http.HandlerFunc(apiHandler.ListProxies)))
	mux.Handle("POST /api/proxies", authMiddleware.RequireAuth(http.HandlerFunc(apiHandler.CreateProxy)))
	mux.Handle("PUT /api/proxies/{id}", authMiddleware.RequireAuth(http.HandlerFunc(apiHandler.UpdateProxy)))
	mux.Handle("DELETE /api/proxies/{id}", authMiddleware.RequireAuth(http.HandlerFunc(apiHandler.DeleteProxy)))
	mux.Handle("GET /api/config/status", authMiddleware.RequireAuth(http.HandlerFunc(apiHandler.ConfigStatus)))
	mux.Handle("POST /api/config/reload", authMiddleware.RequireAuth(http.HandlerFunc(apiHandler.ReloadConfig)))

	log.Printf("Starting NGINX Proxy Manager on %s", addr)
	log.Printf("Stream config: %s", streamPath)
	log.Printf("Database: %s", dbPath)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

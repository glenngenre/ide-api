package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"skwtr-ide-backend/db"
	"skwtr-ide-backend/handlers"
	"skwtr-ide-backend/middleware"
)

func main() {
	db.Init()
	seedAdmin()

	mux := http.NewServeMux()

	// ── Auth ──────────────────────────────────────────────────────────────────
	// Public
	mux.HandleFunc("/v1/auth/login", handlers.Login)

	// Admin-only
	mux.Handle("/v1/auth/register",
		middleware.AdminOnly(http.HandlerFunc(handlers.Register)))

	mux.Handle("/v1/auth/users",
		middleware.AdminOnly(http.HandlerFunc(handlers.ListUsers)))

	mux.Handle("/v1/auth/users/",
		middleware.AdminOnly(http.HandlerFunc(handlers.DeleteUser)))

	mux.Handle("/v1/ai/chat",
		middleware.Auth(http.HandlerFunc(handlers.Chat)))

	mux.Handle("/v1/ai/complete",
		middleware.Auth(http.HandlerFunc(handlers.Complete)))

	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("skwtr-ide-backend listening on %s", addr)
	if err := http.ListenAndServe(addr, corsMiddleware(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func seedAdmin() {
	username := os.Getenv("ADMIN_USERNAME")
	password := os.Getenv("ADMIN_PASSWORD")
	if username == "" || password == "" {
		log.Println("seed: ADMIN_USERNAME/ADMIN_PASSWORD not set, skipping")
		return
	}

	existing, err := db.GetUserByUsername(username)
	if err != nil {
		log.Fatalf("seed: db error: %v", err)
	}
	if existing != nil {
		log.Printf("seed: admin '%s' already exists, skipping", username)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("seed: failed to hash admin password: %v", err)
	}

	if _, err := db.CreateUser(username, string(hash), "admin"); err != nil {
		log.Fatalf("seed: failed to create admin: %v", err)
	}

	log.Printf("seed: admin '%s' created", username)
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := strings.Split(os.Getenv("CORS_ORIGINS"), ",")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowed := false
		for _, o := range allowedOrigins {
			if strings.TrimSpace(o) == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

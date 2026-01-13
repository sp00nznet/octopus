package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sp00nznet/octopus/internal/auth"
	"github.com/sp00nznet/octopus/internal/config"
	"github.com/sp00nznet/octopus/internal/db"
	"github.com/sp00nznet/octopus/internal/scheduler"
)

// Server represents the API server
type Server struct {
	config    *config.Config
	db        *db.Database
	auth      *auth.Authenticator
	scheduler *scheduler.Scheduler
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, database *db.Database, sched *scheduler.Scheduler) *Server {
	return &Server{
		config:    cfg,
		db:        database,
		auth:      auth.New(cfg),
		scheduler: sched,
	}
}

// Router returns the configured HTTP router
func (s *Server) Router() *mux.Router {
	r := mux.NewRouter()

	// Serve static files for the web client
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("../client/static"))))

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Public routes
	api.HandleFunc("/health", s.healthCheck).Methods("GET")
	api.HandleFunc("/auth/login", s.login).Methods("POST")

	// Protected routes
	protected := api.PathPrefix("").Subrouter()
	protected.Use(s.authMiddleware)

	// Source environments
	protected.HandleFunc("/sources", s.listSourceEnvironments).Methods("GET")
	protected.HandleFunc("/sources", s.createSourceEnvironment).Methods("POST")
	protected.HandleFunc("/sources/{id}", s.getSourceEnvironment).Methods("GET")
	protected.HandleFunc("/sources/{id}", s.updateSourceEnvironment).Methods("PUT")
	protected.HandleFunc("/sources/{id}", s.deleteSourceEnvironment).Methods("DELETE")
	protected.HandleFunc("/sources/{id}/sync", s.syncSourceEnvironment).Methods("POST")

	// Target environments
	protected.HandleFunc("/targets", s.listTargetEnvironments).Methods("GET")
	protected.HandleFunc("/targets", s.createTargetEnvironment).Methods("POST")
	protected.HandleFunc("/targets/{id}", s.getTargetEnvironment).Methods("GET")
	protected.HandleFunc("/targets/{id}", s.updateTargetEnvironment).Methods("PUT")
	protected.HandleFunc("/targets/{id}", s.deleteTargetEnvironment).Methods("DELETE")

	// VMs
	protected.HandleFunc("/vms", s.listVMs).Methods("GET")
	protected.HandleFunc("/vms/{id}", s.getVM).Methods("GET")
	protected.HandleFunc("/vms/{id}/estimate", s.estimateVMSize).Methods("POST")

	// Migration jobs
	protected.HandleFunc("/migrations", s.listMigrations).Methods("GET")
	protected.HandleFunc("/migrations", s.createMigration).Methods("POST")
	protected.HandleFunc("/migrations/{id}", s.getMigration).Methods("GET")
	protected.HandleFunc("/migrations/{id}", s.updateMigration).Methods("PUT")
	protected.HandleFunc("/migrations/{id}/cancel", s.cancelMigration).Methods("POST")
	protected.HandleFunc("/migrations/{id}/sync", s.triggerSync).Methods("POST")
	protected.HandleFunc("/migrations/{id}/cutover", s.triggerCutover).Methods("POST")

	// Scheduled tasks
	protected.HandleFunc("/schedules", s.listScheduledTasks).Methods("GET")
	protected.HandleFunc("/schedules", s.createScheduledTask).Methods("POST")
	protected.HandleFunc("/schedules/{id}", s.getScheduledTask).Methods("GET")
	protected.HandleFunc("/schedules/{id}/cancel", s.cancelScheduledTask).Methods("POST")

	// Admin routes
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(s.authMiddleware)
	admin.Use(s.adminMiddleware)

	// Environment variables (admin portal)
	admin.HandleFunc("/env", s.listEnvVariables).Methods("GET")
	admin.HandleFunc("/env", s.createEnvVariable).Methods("POST")
	admin.HandleFunc("/env/{id}", s.updateEnvVariable).Methods("PUT")
	admin.HandleFunc("/env/{id}", s.deleteEnvVariable).Methods("DELETE")

	// Activity logs
	admin.HandleFunc("/logs", s.listActivityLogs).Methods("GET")

	// Users
	admin.HandleFunc("/users", s.listUsers).Methods("GET")
	admin.HandleFunc("/users/{id}", s.getUser).Methods("GET")
	admin.HandleFunc("/users/{id}/admin", s.toggleUserAdmin).Methods("PUT")

	// Web client routes
	r.HandleFunc("/", s.serveIndex).Methods("GET")
	r.HandleFunc("/login", s.serveLogin).Methods("GET")
	r.PathPrefix("/").Handler(http.HandlerFunc(s.serveIndex))

	return r
}

// Response helpers
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// Health check endpoint
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// Serve web client
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../client/templates/index.html")
}

func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../client/templates/login.html")
}

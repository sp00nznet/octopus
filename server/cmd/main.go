package main

import (
	"log"
	"net/http"
	"os"

	"github.com/sp00nznet/octopus/internal/api"
	"github.com/sp00nznet/octopus/internal/config"
	"github.com/sp00nznet/octopus/internal/db"
	"github.com/sp00nznet/octopus/internal/scheduler"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	database, err := db.Initialize(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize scheduler for cutover/failover tasks
	sched := scheduler.New(database)
	go sched.Start()

	// Initialize API server
	server := api.NewServer(cfg, database, sched)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Octopus server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, server.Router()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

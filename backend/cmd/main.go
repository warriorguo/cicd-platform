package main

import (
	"fmt"
	"log"

	"github.com/warriorguo/cicd-platform/backend/internal/api"
	"github.com/warriorguo/cicd-platform/backend/internal/db"
	"github.com/warriorguo/cicd-platform/backend/internal/git"
	"github.com/warriorguo/cicd-platform/backend/internal/k8s"
	"github.com/warriorguo/cicd-platform/backend/pkg/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	database, err := db.Connect(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := db.Migrate(database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize Git client
	gitClient := git.NewClient()

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(cfg.K8s.InCluster, cfg.K8s.KubeConfig, cfg.K8s.PipelineNS)
	if err != nil {
		log.Printf("Warning: Failed to initialize K8s client: %v", err)
		k8sClient = nil // Continue without K8s for development
	}

	// Initialize handlers
	handler := api.NewHandler(database, gitClient, k8sClient, cfg)

	// Reconcile any releases left in deploying/running state from before restart
	handler.ReconcileStaleReleases()

	// Start periodic cleanup of old completed PipelineRuns
	handler.StartPipelineRunCleanupLoop()

	// Setup routes
	router := api.SetupRoutes(handler)

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
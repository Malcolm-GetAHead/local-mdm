package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/api"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/malcolm-getahead/local-mdm/internal/logging"
)

const version = "0.1.0"

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logger := logging.New(cfg.Logging)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	// Print startup banner
	printBanner(cfg)

	// Connect to database
	logger.Info("Connecting to database", "host", cfg.Database.Host, "port", cfg.Database.Port)
	database, err := db.New(cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	logger.Info("Database connection established")

	// Create API server
	server := api.New(cfg, database, logger)

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", "address", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
		if err := server.Start(); err != nil {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutdown signal received")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully")
}

func printBanner(cfg *config.Config) {
	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║              Local MDM Server                         ║")
	fmt.Println("╠═══════════════════════════════════════════════════════╣")
	fmt.Printf("║  Version:     %-40s║\n", version)
	fmt.Printf("║  Listen:      %s:%-33d║\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("║  Database:    %s:%-33d║\n", cfg.Database.Host, cfg.Database.Port)
	fmt.Printf("║  Log Level:   %-40s║\n", cfg.Logging.Level)
	fmt.Println("╚═══════════════════════════════════════════════════════╝")
	fmt.Println()
}

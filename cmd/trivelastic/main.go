package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/truemilk/trivelastic/internal/config"
	"github.com/truemilk/trivelastic/internal/handler"
	"github.com/truemilk/trivelastic/internal/logger"
	"github.com/truemilk/trivelastic/internal/worker"
)

func main() {
	// Initialize logger
	err := logger.Initialize(logger.Config{
		Level:      os.Getenv("LOG_LEVEL"),
		JSONFormat: os.Getenv("LOG_FORMAT") == "json",
	})
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log := logger.GetLogger("main")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to load configuration")
	}

	// Create a buffered channel for requests
	numWorkers := runtime.NumCPU() * 2
	log.Info().
		Int("num_workers", numWorkers).
		Msg("Initializing worker pool")

	requestPool := worker.NewPool(numWorkers)

	// Create and start the server
	log.Info().
		Str("port", cfg.Port).
		Int("workers", numWorkers).
		Msg("Initializing server")

	server := handler.NewServer(cfg, requestPool)
	if err := server.Start(); err != nil {
		log.Fatal().
			Err(err).
			Msg("Server failed to start")
	}
}

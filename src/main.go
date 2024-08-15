package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"unusual-api/src/config"
	"unusual-api/src/listener"
	"unusual-api/src/rpc"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize Ethereum client
	client, err := rpc.NewClient(cfg.RPC_URL)
	if err != nil {
		logger.Fatal("Failed to connect to Ethereum client", zap.Error(err))
	}
	defer client.Close()

	// Initialize event listener
	l := listener.New(client, cfg, logger)

	// Start listening for events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go l.Start(ctx)

	// Wait for interrupt signal to gracefully shut down
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")
}

package main

import (
	"context"
	"ethereum-rpc-pool/handlers"
	"ethereum-rpc-pool/middleware"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	handlers.SetLogger(logger)

	envFile := filepath.Join(".", ".env")
	if _, err := os.Stat(envFile); err == nil {
		logger.Info("loading .env file")
		if err := godotenv.Load(envFile); err != nil {
			logger.Warn("could not load .env file, proceeding with system env", "error", err)
		}
	} else if os.IsNotExist(err) {
		logger.Info("no .env file found, using system environment variables")
	} else {
		logger.Warn("could not check .env file existence", "error", err)
	}

	rpcList := os.Getenv("RPC_LIST")
	if rpcList == "" {
		logger.Error("RPC_LIST environment variable is not defined")
		os.Exit(1)
	}
	handlers.SetRPCs(rpcList)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlers.RPCHandler)
	mux.HandleFunc("/status", handlers.StatusHandler)
	mux.HandleFunc("/healthz", handlers.HealthHandler)
	mux.Handle("/metrics", promhttp.Handler())

	chain := middleware.Recovery(logger)(
		middleware.RequestID(
			middleware.AccessLog(logger)(
				middleware.MaxBodySize(1<<20)(mux),
			),
		),
	)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      chain,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("health check loop panicked", "error", r)
			}
		}()
		handlers.FetchBlockNumber()
	}()

	go func() {
		logger.Info("server started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server stopped")
}

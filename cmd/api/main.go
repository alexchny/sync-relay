package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexchny/sync-relay/internal/adapters/plaid"
	"github.com/alexchny/sync-relay/internal/adapters/postgres"
	"github.com/alexchny/sync-relay/internal/adapters/redis"
	"github.com/alexchny/sync-relay/internal/api/handlers"
	"github.com/alexchny/sync-relay/internal/config"
	"github.com/alexchny/sync-relay/internal/service"
)

func main() {
	// load config
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// setup logger
	var logger *slog.Logger
	if cfg.Env == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	slog.SetDefault(logger)

	slog.Info("starting sync-relay api", "env", cfg.Env, "port", cfg.ServerPort)

	// connect to database
	db, err := postgres.NewDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("failed to close db", "error", err)
		}
	}()
	slog.Info("connected to postgres")

	// connect to redis
	redisClient, err := redis.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			slog.Error("failed to close redis", "error", err)
		}
	}()
	slog.Info("connected to redis")

	// create adapters
	itemRepo := postgres.NewItemRepo(db)
	queueAdapter := redis.NewQueueAdapter(redisClient, "sync:jobs")
	plaidAdapter := plaid.NewAdapter(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnv)

	// create services
	accountService := service.NewAccountService(plaidAdapter, itemRepo, queueAdapter)

	// create handlers
	accountHandler := handlers.NewAccountHandler(accountService)
	webhookHandler := handlers.NewWebhookHandler(plaidAdapter, itemRepo, queueAdapter)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// account onboarding routes
	mux.HandleFunc("/api/link/token", accountHandler.CreateLinkToken)
	mux.HandleFunc("/api/items", accountHandler.ConnectItem)

	// webhook routes
	mux.HandleFunc("/webhooks/plaid", webhookHandler.HandlePlaidWebhook)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.ServerPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	go func() {
		slog.Info("starting server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	slog.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited")
}

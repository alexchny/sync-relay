package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexchny/sync-relay/internal/adapters/plaid"
	"github.com/alexchny/sync-relay/internal/adapters/postgres"
	"github.com/alexchny/sync-relay/internal/adapters/redis"
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

	slog.Info("starting sync-relay worker", "env", cfg.Env)

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

	lockAdapter := redis.NewLockAdapter(redisClient)
	queueAdapter := redis.NewQueueAdapter(redisClient, "sync:jobs")

	plaidClient := plaid.NewAdapter(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnv)

	itemRepo := postgres.NewItemRepo(db)
	txRepo := postgres.NewTransactionRepo(db)

	// prod rate limits for /transactions/sync
	// 2500 req/min per client, 50 req/min per item
	globalLimiter := redis.NewRateLimiter(redisClient, 2500, 1*time.Minute)
	itemLimiter := redis.NewRateLimiter(redisClient, 50, 1*time.Minute)

	syncer := service.NewSyncer(
		itemRepo,
		txRepo,
		plaidClient,
		lockAdapter,
		queueAdapter,
		globalLimiter,
		itemLimiter,
	)

	// start worker loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	slog.Info("starting workers", "count", cfg.WorkerConcurrency)

	for i := 0; i < cfg.WorkerConcurrency; i++ {
		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					job, err := queueAdapter.Dequeue(ctx, 2*time.Second)
					if err != nil {
						slog.Error("queue error", "worker_id", workerID, "error", err)
						time.Sleep(time.Second)
						continue
					}
					if job == nil {
						continue
					}

					slog.Info("processing job", "worker_id", workerID, "item_id", job.ItemID)
					if err := syncer.SyncItem(ctx, job.ItemID); err != nil {
						slog.Error("sync failed", "worker_id", workerID, "item_id", job.ItemID, "error", err)
					}
				}
			}
		}(i)
	}

	// wait for shutdown
	<-stop
	slog.Info("shutdown signal received, stopping workers...")
	cancel()
	time.Sleep(2 * time.Second)
	slog.Info("shutdown complete")
}

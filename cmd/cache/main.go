package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"distributed-cache/internal/cache"
	"distributed-cache/internal/server"
)

const shutdownTimeout = 5 * time.Second

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func main() {
	addr := flag.String("addr", envOr("CACHE_ADDR", ":6380"), "listen address")
	maxEntries := flag.Int("max-entries", envInt("CACHE_MAX_ENTRIES", 1024), "maximum cache entries")
	cleanupInterval := flag.Duration("cleanup-interval", envDuration("CACHE_CLEANUP_INTERVAL", time.Minute), "TTL janitor interval (0 disables)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c, err := cache.NewCacheWithOptions(cache.Options{
		MaxEntries:      *maxEntries,
		CleanupInterval: *cleanupInterval,
	})
	if err != nil {
		logger.Error("failed to create cache", "err", err)
		os.Exit(1)
	}

	srv := server.New(c, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(*addr) }()

	var serveErr error
	select {
	case serveErr = <-errCh:
		// The accept loop exited without a signal (fatal bind/accept error, or
		// the listener was closed by something else).
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "err", err)
			serveErr = err
		} else {
			serveErr = <-errCh // wait for the accept loop to observe the close
		}
	}

	c.Close()

	if serveErr != nil {
		logger.Error("server exited with error", "err", serveErr)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/SoftwareBeesy/work-platform-agent/internal/agent"
	"github.com/SoftwareBeesy/work-platform-agent/internal/config"
	"github.com/SoftwareBeesy/work-platform-agent/internal/queue"
	"github.com/SoftwareBeesy/work-platform-agent/internal/transport"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	eventQueue, err := queue.Open(cfg.QueuePath)
	if err != nil {
		logger.Error("open event queue", "err", err)
		os.Exit(1)
	}
	defer eventQueue.Close()

	client, err := transport.New(cfg)
	if err != nil {
		logger.Error("create transport client", "err", err)
		os.Exit(1)
	}

	runner := agent.NewRunner(cfg, client, eventQueue, logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("farm agent starting",
		"farm_id", cfg.FarmID,
		"control_plane", cfg.ControlPlaneURL,
		"queue_path", cfg.QueuePath,
	)

	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("agent stopped with error", "err", err)
		os.Exit(1)
	}
	logger.Info("farm agent stopped")
}

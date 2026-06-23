package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/revunix/defqon1-recorder/internal/config"
	"github.com/revunix/defqon1-recorder/internal/controller"
	"github.com/revunix/defqon1-recorder/internal/mixlr"
	"github.com/revunix/defqon1-recorder/internal/recorder"
	"github.com/revunix/defqon1-recorder/internal/status"
	"github.com/revunix/defqon1-recorder/internal/timetable"
	"github.com/revunix/defqon1-recorder/internal/tui"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg := config.Default()

	logCh := make(chan tui.LogMessage, 2048)
	logger := tui.NewLogger(logCh)

	if err := os.MkdirAll(cfg.RecordingsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create recordings dir: %s\n", err)
		os.Exit(1)
	}
	if abs, err := filepath.Abs(cfg.RecordingsDir); err == nil {
		logger.Info(fmt.Sprintf("Recordings saved in: %s", abs))
	}

	tt, err := timetable.Load(cfg.TimetablePath)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to load timetable: %s", err))
		tt = &timetable.Timetable{}
	} else {
		logger.Info(fmt.Sprintf("Timetable loaded with %d sets.", tt.Size()))
	}

	client := mixlr.New(cfg.APIBaseURL)
	rec := recorder.New(cfg.RecordingsDir, cfg.StalledTimeout, cfg.ToolsDir, logger)
	reg := status.New(cfg.Channels)
	ctrl := controller.New(client, rec, tt, reg, logger, cfg.Channels)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ui := tui.New(cfg, rec, tt, reg, logCh)

	go ctrl.Run(ctx, cfg.CheckInterval, cfg.StalledCheckInterval)
	go ui.RunRefresh(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		ui.Stop()
	}()

	logger.Info(fmt.Sprintf("--- Starting Stream Recorder (v%s) ---", version))

	if err := ui.Run(); err != nil {
		logger.Error(fmt.Sprintf("UI error: %s", err))
	}

	cancel()
	ui.Close()
	rec.StopAll(10 * time.Second)
}

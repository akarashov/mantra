// Package main - точка входа в приложение MANTRA, которая инициализирует логгер, обрабатывает сигналы завершения и запускает основное приложение
package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/akarashov/mantra/internal/app"
	"github.com/akarashov/mantra/pkg/logger"
	"github.com/akarashov/mantra/pkg/utils"
)

// main - основная функция, которая запускает приложение MANTRA
func main() {
	log_level := utils.Getenv("LOG_LEVEL", "info")

	log := logger.New(log_level)
	log.Info("setting log level, using environment variable LOG_LEVEL,", "$LOG_LEVEL", log_level)

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	log.Info("starting MANTRA")
	application, err := app.New(log)
	if err != nil {
		log.Fatal("failed to start application", "error", err)
	}

	log.Info("running MANTRA")
	err = application.Run(ctx)
	if err != nil {
		log.Fatal("failed to run application", "error", err)
	}

	<-ctx.Done()
	log.Info("MANTRA stopped gracefully")
}

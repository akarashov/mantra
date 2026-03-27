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
	logLevel := utils.Getenv("LOG_LEVEL", "info")

	log := logger.New(logLevel)
	log.Info("setting log level, using environment variable LOG_LEVEL,", "$LOG_LEVEL", logLevel)

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	log.Info("starting MANTRA")
	application, err := app.New(log)
	if err != nil {
		log.Fatal("failed to start application", "error", err)
	}

	log.Info("running MANTRA")
	application.Run(ctx)

	<-ctx.Done()
	log.Info("MANTRA stopped gracefully")
}

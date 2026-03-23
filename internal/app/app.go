// Package app - основной пакет приложения, который инициализирует все компоненты и запускает бота
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/akarashov/mantra/internal/clients/chat"
	"github.com/akarashov/mantra/internal/clients/speech"
	"github.com/akarashov/mantra/internal/config"
	"github.com/akarashov/mantra/internal/handlers"
	"github.com/akarashov/mantra/internal/repository/postgres"
	chatsvc "github.com/akarashov/mantra/internal/services/chat"
	speechsvc "github.com/akarashov/mantra/internal/services/speech"
	"github.com/akarashov/mantra/internal/services/user"
	"github.com/akarashov/mantra/pkg/logger"
	"github.com/akarashov/mantra/pkg/utils"

	"gopkg.in/telebot.v3"
)

// App - основное приложение, которое инициализирует все компоненты и запускает бота
type App struct {
	log     *logger.Logger
	cfg     *config.Config
	bot     *telebot.Bot
	handler *handlers.Handler
}

// New создаёт новый экземпляр приложения, инициализируя все компоненты
func New(log *logger.Logger) (*App, error) {

	cfgPath := utils.Getenv("CONFIG_PATH", "default.yaml")
	log.Info("loading configuration from file using environment variable CONFIG_PATH", "$CONFIG_PATH", cfgPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	log.Info("creating telegram bot", "token_set", cfg.Telegram.Token != "")
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	log.Info("creating postgres repository", "dsn_set", cfg.Database.DSN != "")
	repo, err := postgres.New(cfg.Database, log)
	if err != nil {
		return nil, fmt.Errorf("create postgres repository: %w", err)
	}

	log.Info("initializing speech recognition client", "grpc_endpoint", cfg.Salute.GRPCEndpoint)
	speechClient, err := initSpeechClient(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("create speech gRPC client: %w", err)
	}

	log.Info("initializing chat with AI client", "api_url", cfg.GigaChat.APIURL)
	chatClient := initChatClient(cfg, log)

	log.Info("initializing services")
	userService, speechService, chatService := initServices(repo, speechClient, chatClient, log)

	log.Info("creating handlers")
	handler := handlers.New(
		bot,
		userService,
		speechService,
		chatService,
		log,
	)

	return &App{
		log:     log,
		cfg:     cfg,
		bot:     bot,
		handler: handler,
	}, nil
}

// Run запускает приложение, регистрируя обработчики и запуская бота, а также обрабатывая сигналы завершения для graceful shutdown
func (a *App) Run(ctx context.Context) error {
	a.handler.Register()
	errCh := make(chan error, 1) // Канал для ошибок при запуске бота с буфером 1, чтобы избежать блокировки
	go func() {
		a.log.Info("starting telegram bot poller...")
		a.bot.Start()
	}()

	// Ожидание сигнала завершения
	<-ctx.Done()

	a.log.Info("stopping telegram bot...")
	a.bot.Stop()

	// Ожидание завершения бота с таймаутом
	select {
	case err := <-errCh:
		return err
	case <-time.After(10 * time.Second):
		return fmt.Errorf("bot shutdown timeout")
	default:
		return nil
	}
}

// initSpeechClient инициализирует клиент для распознавания речи
func initSpeechClient(cfg *config.Config, log *logger.Logger) (*speech.Client, error) {
	return speech.NewClient(
		cfg.Salute.GRPCEndpoint,
		cfg.Salute.ClientID,
		cfg.Salute.ClientSecret,
		cfg.Salute.Scope,
		cfg.Salute.AuthURL,
		cfg.Salute.ModelURI,
		cfg.Salute.CertPath,
		cfg.Salute.PollInterval,
		log.With("component", "speech_client"),
		cfg.Salute.GRPCInsecure,
	)
}

// initChatClient инициализирует клиент для ИИ чата
func initChatClient(cfg *config.Config, log *logger.Logger) *chat.Client {
	return chat.NewClient(
		cfg.GigaChat.ClientID,
		cfg.GigaChat.ClientSecret,
		cfg.GigaChat.Scope,
		cfg.GigaChat.AuthURL,
		cfg.GigaChat.APIURL,
		cfg.GigaChat.Model,
		cfg.GigaChat.CertPath,
		log.With("component", "chat_client"),
	)
}

// initServices инициализирует все сервисы
func initServices(repo *postgres.Postgres, speechClient *speech.Client, chatClient *chat.Client, log *logger.Logger) (*user.Service, *speechsvc.Service, *chatsvc.Service) {
	userService := user.NewService(repo, log)
	chatService := chatsvc.NewService(chatClient, repo, log)
	speechService := speechsvc.NewService(speechClient, repo, log, chatService)
	return userService, speechService, chatService
}

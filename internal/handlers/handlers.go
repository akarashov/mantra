// Package handlers содержит логику обработки сообщений от Telegram бота, включая команды и медиа, а также взаимодействие с сервисами для управления пользователями, распознавания речи и ведения чатов.
package handlers

import (
	"sync"

	"github.com/akarashov/mantra/internal/services/chat"
	"github.com/akarashov/mantra/internal/services/speech"
	"github.com/akarashov/mantra/internal/services/user"
	"github.com/akarashov/mantra/pkg/logger"

	"gopkg.in/telebot.v3"
)

type UserSession struct {
	State string
	Data  map[string]any
}

// Handler - структура, которая содержит все зависимости для обработки сообщений от Telegram бота
type Handler struct {
	bot           *telebot.Bot    // Telegram бот
	userService   *user.Service   // Сервис для управления пользователями
	speechService *speech.Service // Сервис для распознавания речи
	chatService   *chat.Service   // Сервис для ведения чатов с AI
	log           *logger.Logger  // Логгер для логирования событий и ошибок
	sessions   map[int64]*UserSession // sessions хранит состояние пользователя между сообщениями.
	sessionsMu sync.RWMutex
}

// New создаёт новый экземпляр Handler, инициализируя все зависимости
func New(
	bot *telebot.Bot,
	userService *user.Service,
	speechService *speech.Service,
	chatService *chat.Service,
	log *logger.Logger,
) *Handler {
	return &Handler{
		bot:           bot,
		userService:   userService,
		speechService: speechService,
		chatService:   chatService,
		log:           log,
		sessions:      make(map[int64]*UserSession),
	}
}

// Register регистрирует все обработчики для различных типов сообщений и команд Telegram бота
func (h *Handler) Register() {
	// Текстовые команды
	h.bot.Handle("/start", h.handleStart)
	h.bot.Handle("/list", h.handleList)
	h.bot.Handle("/get", h.handleGet)
	h.bot.Handle("/find", h.handleFind)
	h.bot.Handle("/chat", h.handleChatQuestion)
	h.bot.Handle("/help", h.handleHelp)

	// Обработчики медиа
	h.bot.Handle(telebot.OnVoice, h.handleVoice)
	h.bot.Handle(telebot.OnAudio, h.handleAudio)

	// Обработчик текста по умолчанию
	h.bot.Handle(telebot.OnText, h.handleText)
}

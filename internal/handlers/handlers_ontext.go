package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/akarashov/mantra/pkg/utils"

	"gopkg.in/telebot.v3"
)

type UserSession struct {
	State string
	Data  map[string]any
}

var sessions = make(map[int64]*UserSession)

// handleStart - регистрация пользователя
func (h *Handler) handleStart(c telebot.Context) error {
	sender := c.Sender()
	if err := h.userService.Register(context.Background(), sender.ID, sender.Username); err != nil {
		h.log.Error("failed to register user", "user_id", sender.ID, "error", err)
		return c.Reply("Ошибка регистрации. Попробуйте позже.")
	}
	menu := &telebot.ReplyMarkup{
		ResizeKeyboard: true,
		ReplyKeyboard: [][]telebot.ReplyButton{
			{{Text: "Список встреч"}, {Text: "Поиск"}},
			{{Text: "Получить встречу"}}, {{Text: "Чат с ИИ"}},
		},
	}
	return c.Reply(
		fmt.Sprintf("Привет, %s!\n\n"+
			"Я — MANTRA, Meetings Assistant for Notetaking Transcription Review and Analysis (Ассистент для встреч: запись, расшифровка, обзор и анализ).\n"+
			"Харе Кришна, Харе Кришна, Кришна Кришна, Харе Харе\nХаре Рама, Харе Рама, Рама Рама, Харе Харе\n\n"+
			"Просто отправьте мне голосовое сообщение или аудиофайл\n\n"+
			"  /help — показать помощь\n",
			utils.SafeString(sender.FirstName)),
		menu,
	)
}

// handleText - обработка обычных текстовых сообщений
func (h *Handler) handleText(c telebot.Context) error {
	userID := c.Sender().ID
	if session, exists := sessions[userID]; exists {
		switch session.State {
		case "awaiting_search":
			return h.handleFind(c)
		case "awaiting_ai_question":
			return h.handleChatQuestion(c)
		case "awaiting_get_meeting":
			return h.handleGet(c)
		default:
			delete(sessions, userID)
			return c.Send("Сессия сброшена. Пожалуйста, используйте кнопки меню или /help для справки.")
		}
	}
	switch c.Text() {
	case "Список встреч":
		return h.handleList(c)
	case "Поиск":
		sessions[userID] = &UserSession{
			State: "awaiting_search",
			Data:  make(map[string]any),
		}
		return c.Send("Введите текст для поиска:")
	case "Чат с ИИ":
		sessions[userID] = &UserSession{
			State: "awaiting_ai_question",
			Data:  make(map[string]any),
		}
		return c.Send("Задайте вопрос ИИ:")
	case "Получить встречу":
		sessions[userID] = &UserSession{
			State: "awaiting_get_meeting",
			Data:  make(map[string]any),
		}
		return c.Send("Введите ID встречи:")
	default:
		return c.Send("Используйте кнопки меню или /help для справки.")
	}
}

// handleList - список встреч пользователя
func (h *Handler) handleList(c telebot.Context) error {
	// TODO: пагинация вместо 10 последних
	h.log.Debug("fetching meetings for user", "user_id", c.Sender().ID)
	meetings, err := h.userService.GetMeetings(context.Background(), c.Sender().ID, 10, 0)
	if err != nil {
		h.log.Error("failed to get meetings", "user_id", c.Sender().ID, "error", err)
		return c.Reply("Ошибка получения списка встреч")
	}
	if len(meetings) == 0 {
		return c.Reply("У вас пока нет сохранённых встреч.\n" +
			"Отправьте голосовое сообщение для начала!")
	}
	var response strings.Builder
	fmt.Fprintf(&response, "Ваши последние встречи:\n\n")
	for _, m := range meetings {
		fmt.Fprintf(&response,
			"Встреча id: %d от %s\nИтоги: %s\n\n",
			m.ID,
			utils.FormatDateTime(m.CreatedAt),
			utils.TruncateString(m.Summary, 150))
	}
	fmt.Fprintf(&response, "Используйте /get <id> для просмотра деталей")
	return c.Reply(response.String())
}

// handleGet - получение встречи по ID
func (h *Handler) handleGet(c telebot.Context) error {
	h.log.Debug("getting meeting details", "user_id", c.Sender().ID, "payload", c.Message().Payload)
	meetingID := utils.ParseInt64(c.Message().Payload)
	if meetingID == 0 {
		return c.Reply("Укажите id встречи, например:\n/get 13")
	}
	meeting, err := h.userService.GetMeeting(context.Background(), c.Sender().ID, meetingID)
	if err != nil {
		h.log.Error("failed to get meeting", "meeting_id", meetingID, "error", err)
		return c.Reply("Ошибка получения встречи")
	}
	if meeting == nil {
		return c.Reply("Встреча не найдена")
	}
	response := fmt.Sprintf("Встреча id: %d от %s\n"+
		"Длительность: %s\n\n"+
		"Итоги:\n%s\n\n"+
		"Транскрипция:\n%s\n\n",
		meeting.ID,
		utils.FormatDateTime(meeting.CreatedAt),
		utils.FormatDuration(meeting.Duration),
		utils.TruncateString(meeting.Summary, 1000),
		utils.TruncateString(meeting.Transcript, 3000))
	return c.Reply(response)
}

// handleFind - поиск по ключевым словам
func (h *Handler) handleFind(c telebot.Context) error {
	query := strings.TrimSpace(c.Message().Payload)
	if query == "" {
		return c.Reply("Введите поисковый запрос, например:\n/find договорённости сроки")
	}
	meetings, err := h.userService.SearchMeetings(context.Background(), c.Sender().ID, query)
	if err != nil {
		h.log.Error("search failed", "user_id", c.Sender().ID, "query", query, "error", err)
		return c.Reply("Ошибка поиска")
	}
	if len(meetings) == 0 {
		return c.Reply(fmt.Sprintf("Ничего не найдено по запросу \"%s\"", query))
	}
	var response strings.Builder
	fmt.Fprintf(&response, "Найдено встреч: %d\n\n", len(meetings))
	for _, m := range meetings {
		preview := utils.HighlightKeywords(m.Transcript, query, 200)
		fmt.Fprintf(&response, "Встреча id: %d от %s\n%s\n\n",
			m.ID,
			utils.FormatDateTime(m.CreatedAt),
			preview)
	}
	return c.Reply(response.String(), &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
}

// handleChatQuestion - вопрос к ИИ-ассистенту
func (h *Handler) handleChatQuestion(c telebot.Context) error {
	question := strings.TrimSpace(c.Message().Payload)
	if question == "" {
		return c.Reply("Задайте вопрос ИИ, например:\n/chat Какие сроки по проекту?")
	}
	processingMsg, err := c.Bot().Send(c.Chat(), "Ом Трайамбакам Яджамахе Сугандхим Пушти Вардханам Урварукамива Бандханан Мритьор Мукшийя Мамритат...")
	if err != nil {
		return err
	}
	answer, err := h.chatService.AskQuestion(context.Background(), c.Sender().ID, question)
	if err != nil {
		h.log.Error("chat service error", "user_id", c.Sender().ID, "error", err)
		h.bot.Edit(processingMsg, "Ошибка ИИ-ассистента. Попробуйте позже.")
		return nil
	}
	formatted := fmt.Sprintf("%s", answer)
	if len(formatted) > 4000 {
		formatted = formatted[:4000] + "..."
	}
	h.bot.Edit(processingMsg, formatted)
	return nil
}

// handleHelp - справка
func (h *Handler) handleHelp(c telebot.Context) error {
	helpText := "# Справка по Mantra\n\n" +
		"**Как использовать:**\n" +
		"1. Отправьте голосовое сообщение или аудиофайл\n" +
		"2. Бот автоматически расшифрует и проанализирует встречу\n" +
		"3. Получите краткую выжимку и возможность задать вопросы\n\n" +
		"**Команды:**\n" +
		"  /start — начать работу с ботом\n" +
		"  /list — показать список встреч (последние 10)\n" +
		"  /get <id> — показать детали встречи по ID\n" +
		"  /find <слова> — поиск по содержимому встреч\n" +
		"  /chat <вопрос> — задать вопрос ИИ по вашим встречам\n" +
		"  /help — эта справка\n\n" +
		"**Советы:**\n" +
		"Используйте чёткие формулировки в поиске\n" +
		"Вопросы к ИИ лучше задавать конкретно: «Какие сроки по проекту?»\n" +
		"Чем меньше аудиофайлы, тем быстрее они обрабатываютсяё\n\n"
	return c.Reply(helpText, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
}

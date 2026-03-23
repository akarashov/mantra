package chat

// ChatRequest структура запроса к API чата
type ChatRequest struct {
	Model    string    `json:"model"`            // Модель чата
	Messages []Message `json:"messages"`         // Набор сообщений
	Stream   bool      `json:"stream,omitempty"` // Флаг потока
}

// Message представляет одно сообщение в диалоге
type Message struct {
	Role    string `json:"role"`    // Роль: user|assistant|system|function
	Content string `json:"content"` // Тело сообщения
}

// ChatResponse структура ответа API чата
type ChatResponse struct {
	Choices []Choice `json:"choices"` // Набор ответов
}

// Choice представляет один ответ модели
type Choice struct {
	Message Message `json:"message"` // Сообщение
}

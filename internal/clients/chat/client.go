// package chat представляет собой клиета для работы с GigaChat по REST Api
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/akarashov/mantra/internal/clients/auth"
	"github.com/akarashov/mantra/pkg/logger"
)

// Client предоставляет интерфейс к GigaChat API
type Client struct {
	apiURL string           // URL для рботы с GigaChat
	model  string           // модель для распознования
	log    *logger.Logger   // logger
	auth   *auth.AuthClient // oAuth client
}

// NewClient создаёт новый клиент GigaChat
func NewClient(clientID, clientSecret, scope, authURL, apiURL, model, certPath string, logger *logger.Logger) *Client {
	authClient, err := auth.NewAuthClient(clientID, clientSecret, scope, authURL, certPath, logger)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to create auth client", "error", err)
		}
		return nil
	}
	return &Client{
		apiURL: apiURL,
		model:  model,
		log:    logger,
		auth:   authClient,
	}
}

// GetSummary сделай мне резюме. Не могу, я стесняюсь
func (c *Client) GetSummary(ctx context.Context, text string) (string, error) {
	c.log.Debug("requesting summary from GigaChat", "text_length", len(text))
	prompt := fmt.Sprintf(`Сгенерируй краткое содержание на русском языке по тексту: """%s""" и выдели основные идеи`, text)
	return c.sendMessage(ctx, prompt)
}

// AskQuestion задаёт вопрос модели с учётом контекста
func (c *Client) AskQuestion(ctx context.Context, question, meetingContext string) (string, error) {
	c.log.Debug("asking question to GigaChat", "question", question, "context_length", len(meetingContext))
	prompt := fmt.Sprintf(`Ответь на вопрос """%s""" по русски,`+
		`используя информацию из контекста """%s""",`+
		`если информации недостаточно, не выдумывай`, question, meetingContext)
	return c.sendMessage(ctx, prompt)
}

// sendMessage отправляет сообщение модели и получает ответ
func (c *Client) sendMessage(ctx context.Context, content string) (string, error) {
	// За генерацию текста и изображений отвечает запрос POST /chat/completions.
	// С помощью запросов на генерацию вы можете решать самые разные задачи:
	// переводить, исправлять и стилизовать текст, генерировать краткое содержание статей
	// и выделять из них основные идеи, создавать изображения и многое другое.
	const restApiGigaChatHandle = "/chat/completions"
	// system — системный промпт, который задает роль модели, например, должна модель отвечать как академик или как школьник;
	//assistant — ответ модели;
	//user — сообщение пользователя;
	//function — сообщение с результатом работы пользовательской функции.
	//           В сообщении с этой ролью передавайте результаты работы
	//           функции в поле content в форме валидного JSON-объекта, обернутого в строку.
	const role = "user"
	const srole = "system"
	// Промпт Суммаризация
	const scontent = "Ты - умный ИИ-ассистент, в задачу которого входит суммаризация и реферирование длинных текстов.\n" +
		"## Основные правила:\n" +
		"- Входные данные - текст. На выходе ты должен предоставить краткий текст, в котором содержатся основные идеи исходного текста.\n" +
		"- Избегай длинных и сложных предложений. \n" +
		"- Объём сокращённого текста должен составлять не более 1/5 от исходного текста.\n" +
		"- Сохраняй основной смысл документа. \n" +
		"- Ни один фрагмент сокращённого текста не должен искажать смысл исходного текста.\n" +
		"- Не включай информацию, не являющуюся важной для текста: избегай выводов, оценок, примеров и цитат.\n" +
		"Представь ответ в виде структурированного текста с абзацами. " +
		"Каждый абзац содержит одну основную мысль. " +
		"Используй форматирование для Telegram, а не Markdown. " +
		"Текст должен быть удобочитаемым и чётко разделённым."
	// Режим получения потока токенов поможет обрабатывать ответ GigaChat по мере его генерации.
	const steam = false
	reqBody := ChatRequest{
		Model: c.model,
		Messages: []Message{
			{Role: srole, Content: scontent},
			{Role: role, Content: content},
		},
		Stream: false,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+restApiGigaChatHandle, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.auth.Do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api error [%d]: %s", resp.StatusCode, string(body))
	}
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from GigaChat")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// Package chat бизнес логика работы с ИИ
package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/akarashov/mantra/internal/clients/chat"
	"github.com/akarashov/mantra/internal/repository"
	"github.com/akarashov/mantra/pkg/logger"
)

// Service предоставляет бизнес-логику для взаимодействия с ИИ-ассистентом
type Service struct {
	client *chat.Client
	repo   repository.Repository
	log    *logger.Logger
}

// NewService создаёт новый сервис чата
func NewService(
	client *chat.Client,
	repo repository.Repository,
	log *logger.Logger,
) *Service {
	return &Service{
		client: client,
		repo:   repo,
		log:    log,
	}
}

// AskQuestion задаёт вопрос ИИ с контекстом из встреч пользователя
func (s *Service) AskQuestion(ctx context.Context, userID int64, question string) (string, error) {
	s.log.Info("processing chat question",
		"user_id", userID,
		"question_length", len(question))
	// Берём последние 3 встречи для баланса между контекстом и лимитом токенов
	meetings, err := s.repo.GetMeetingsByUser(ctx, repository.ListQuery{
		UserID: userID,
		Limit:  3,
		Offset: 0,
	})
	if err != nil {
		return "", fmt.Errorf("get user meetings: %w", err)
	}
	var contextBuilder strings.Builder
	for _, m := range meetings {
		fmt.Fprintf(&contextBuilder, "=== Встреча #%d от %s ===\n", m.ID, m.CreatedAt.Format("02.01.2006"))
		transcript := m.Transcript
		if len(transcript) > 2000 {
			transcript = transcript[:2000] + "\n[...]"
		}
		contextBuilder.WriteString(transcript)
		contextBuilder.WriteString("\n\n")
		if contextBuilder.Len() > 10000 {
			s.log.Warn("context truncated due to size limit", "user_id", userID)
			break
		}
	}
	contextText := contextBuilder.String()
	if contextText == "" {
		contextText = "У пользователя пока нет сохранённых встреч."
	}
	answer, err := s.client.AskQuestion(ctx, question, contextText)
	if err != nil {
		return "", fmt.Errorf("get answer from model: %w", err)
	}
	s.log.Debug("chat response received",
		"user_id", userID,
		"answer_length", len(answer))
	return answer, nil
}

// GetSummary запрашивает выжимку текста (используется speech-сервисом)
func (s *Service) GetSummary(ctx context.Context, text string) (string, error) {
	// Экономия токенов
	limitedText := text
	if len(limitedText) > 8000 {
		limitedText = limitedText[:8000]
	}
	summary, err := s.client.GetSummary(ctx, limitedText)
	if err != nil {
		return "", fmt.Errorf("generate summary: %w", err)
	}
	return summary, nil
}

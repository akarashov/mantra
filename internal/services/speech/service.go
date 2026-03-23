package speech

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/akarashov/mantra/internal/clients/speech"
	"github.com/akarashov/mantra/internal/repository"
	"github.com/akarashov/mantra/internal/services/chat"
	"github.com/akarashov/mantra/pkg/logger"
)

// Service предоставляет бизнес-логику обработки речи (в памяти)
type Service struct {
	client      *speech.Client
	repo        repository.Repository
	log         *logger.Logger
	chatService *chat.Service
}

// ProcessInput параметры для обработки встречи (без пути к файлу!)
type ProcessInput struct {
	UserID         int64     // Идентификатор пользователя Telegram
	AudioData      io.Reader // аудио в памяти
	FileName       string
	ContentType    string // MIME-type: "audio/ogg", etc.
	DurationMs     int64  // длительность в миллисекундах
	TelegramFileID string // для отслеживания
}

// ProcessResult результат обработки
type ProcessResult struct {
	MeetingID  int64
	Transcript string
	Summary    string
	DurationMs int64
	CreatedAt  time.Time
}

// NewService создаёт сервис
func NewService(
	client *speech.Client,
	repo repository.Repository,
	log *logger.Logger,
	chatService *chat.Service,
) *Service {
	return &Service{
		client:      client,
		repo:        repo,
		log:         log,
		chatService: chatService,
	}
}

// ProcessMeeting обрабатывает аудио полностью в памяти
func (s *Service) ProcessMeeting(ctx context.Context, input *ProcessInput) (*ProcessResult, error) {
	s.log.Info("starting in-memory meeting processing",
		"user_id", input.UserID,
		"file", input.FileName,
		"content_type", input.ContentType,
		"duration_ms", input.DurationMs,
		"telegram_file_id", input.TelegramFileID,
	)
	transcript, err := s.client.AsyncTranscribe(ctx, input.AudioData, input.FileName, input.ContentType, input.DurationMs)
	if err != nil {
		return nil, fmt.Errorf("transcribe audio: %w", err)
	}
	s.log.Debug("transcription completed",
		"user_id", input.UserID,
		"transcript_length", len(transcript))
	meeting := &repository.Meeting{
		UserID:         input.UserID,
		TelegramFileID: input.TelegramFileID,
		OriginalName:   input.FileName,
		Duration:       int(input.DurationMs / 1000), // конвертируем в секунды
		Transcript:     transcript,
		Summary:        "", // заполним ниже после генерации выжимки
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	meeting, err = s.repo.CreateMeeting(ctx, meeting)
	if err != nil {
		return nil, fmt.Errorf("create meeting record: %w", err)
	}
	summaryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	summary, err := s.generateSummary(summaryCtx, transcript)
	if err != nil {
		s.log.Warn("summary generation failed",
			"meeting_id", meeting.ID, "error", err)
		summary = "Выжимка не сгенерирована"
	}
	_ = s.repo.UpdateMeeting(context.Background(), meeting.ID, input.UserID, transcript, summary)
	s.log.Info("meeting processing completed",
		"meeting_id", meeting.ID,
		"user_id", input.UserID,
		"transcript_len", len(transcript))
	return &ProcessResult{
		MeetingID:  meeting.ID,
		Transcript: transcript,
		Summary:    summary,
		DurationMs: input.DurationMs,
		CreatedAt:  meeting.CreatedAt,
	}, nil
}

// generateSummary запрашивает выжимку у чат-сервиса
func (s *Service) generateSummary(ctx context.Context, transcript string) (string, error) {
	limitedText := transcript
	if len(limitedText) > 8000 {
		limitedText = limitedText[:8000] + "\n[... продолжение ...]"
	}
	return s.chatService.GetSummary(ctx, limitedText)
}

// ProcessMeetingFromBytes удобная обёртка для обработки []byte
func (s *Service) ProcessMeetingFromBytes(ctx context.Context, userID int64, audioBytes []byte, fileName, contentType string, durationMs int64, telegramFileID string) (*ProcessResult, error) {
	return s.ProcessMeeting(ctx, &ProcessInput{
		UserID:         userID,
		AudioData:      bytes.NewReader(audioBytes),
		FileName:       fileName,
		ContentType:    contentType,
		DurationMs:     durationMs,
		TelegramFileID: telegramFileID,
	})
}

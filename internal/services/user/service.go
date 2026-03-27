package user

import (
	"context"
	"fmt"

	"github.com/akarashov/mantra/internal/repository"
	"github.com/akarashov/mantra/pkg/logger"
)

// Service предоставляет бизнес-логику для работы с пользователями
type Service struct {
	userRepo        repository.Crud[repository.User, int64]
	meetingRepo     repository.Crud[repository.Meeting, repository.MeetingKey]
	meetingSearcher repository.Searcher[repository.Meeting, repository.ListQuery, repository.SearchQuery, repository.MeetingWithRank]
	log             *logger.Logger
}

// NewService создаёт новый пользовательский сервис
func NewService(
	userRepo repository.Crud[repository.User, int64],
	meetingRepo repository.Crud[repository.Meeting, repository.MeetingKey],
	meetingSearcher repository.Searcher[repository.Meeting, repository.ListQuery, repository.SearchQuery, repository.MeetingWithRank],
	log *logger.Logger,
) *Service {
	return &Service{
		userRepo:        userRepo,
		meetingRepo:     meetingRepo,
		meetingSearcher: meetingSearcher,
		log:             log,
	}
}

// Register регистрирует нового пользователя или обновляет данные существующего
func (s *Service) Register(ctx context.Context, telegramID int64, username string) error {
	_, err := s.userRepo.Create(ctx, &repository.User{TelegramID: telegramID, Username: username})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	s.log.Info("user registered/updated",
		"telegram_id", telegramID,
		"username", username)
	return nil
}

// GetMeetings получает список встреч пользователя с пагинацией
func (s *Service) GetMeetings(ctx context.Context, userID int64, limit, offset int) ([]repository.Meeting, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	meetings, err := s.meetingSearcher.List(ctx, repository.ListQuery{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("get meetings: %w", err)
	}
	s.log.Debug("meetings fetched", "user_id", userID, "count", len(meetings))
	return meetings, nil
}

// GetMeeting получает конкретную встречу с проверкой прав доступа
func (s *Service) GetMeeting(ctx context.Context, userID, meetingID int64) (*repository.Meeting, error) {
	meeting, err := s.meetingRepo.Read(ctx, repository.MeetingKey{ID: meetingID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("get meeting: %w", err)
	}
	if meeting == nil {
		s.log.Warn("meeting not found or access denied",
			"user_id", userID,
			"meeting_id", meetingID)
		return nil, nil
	}
	return meeting, nil
}

// SearchMeetings выполняет поиск по встречам пользователя
func (s *Service) SearchMeetings(ctx context.Context, userID int64, query string) ([]repository.Meeting, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}
	results, err := s.meetingSearcher.Search(ctx, repository.SearchQuery{
		UserID: userID,
		Query:  query,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("search meetings: %w", err)
	}
	// Конвертируем результат в стандартный формат
	meetings := make([]repository.Meeting, len(results))
	for i, r := range results {
		meetings[i] = r.Meeting
	}
	s.log.Debug("search completed",
		"user_id", userID,
		"query", query,
		"found", len(meetings))

	return meetings, nil
}

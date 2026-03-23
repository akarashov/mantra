// Package repository определяет интерфейс для работы с данными пользователей и встреч (создание, получение, обновление, поиск)
package repository

import (
	"context"
)

// Repository определяет интерфейс для работы с данными пользователей и встреч (создание, получение, обновление, поиск)
type Repository interface {
	// Пользователи
	CreateUser(ctx context.Context, telegramID int64, username string) (*User, error)
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*User, error)

	// Встречи
	CreateMeeting(ctx context.Context, meeting *Meeting) (*Meeting, error)
	GetMeetingByID(ctx context.Context, meetingID, userID int64) (*Meeting, error)
	GetMeetingsByUser(ctx context.Context, query ListQuery) ([]Meeting, error)
	UpdateMeeting(ctx context.Context, meetingID, userID int64, transcript, summary string) error

	// Поиск
	SearchMeetings(ctx context.Context, query SearchQuery) ([]MeetingWithRank, error)
}

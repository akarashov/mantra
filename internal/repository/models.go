package repository

import "time"

// User представляет пользователя системы
type User struct {
	ID         int64     `db:"id"`          // Внутренний ID в БД
	TelegramID int64     `db:"telegram_id"` // Telegram ID пользователя
	Username   string    `db:"username"`    // Telegram username
	CreatedAt  time.Time `db:"created_at"`  // Время создания записи
}

// Meeting представляет записанную встречу
type Meeting struct {
	ID             int64     `db:"id"`               // Внутренний ID в БД
	UserID         int64     `db:"user_id"`          // Telegram ID пользователя
	TelegramFileID string    `db:"telegram_file_id"` // ID файла в Telegram
	OriginalName   string    `db:"original_name"`    // Оригинальное имя файла
	Duration       int       `db:"duration"`         // Длительность в секундах
	Transcript     string    `db:"transcript"`       // Транскрипция встречи
	Summary        string    `db:"summary"`          // Выжимка встречи
	CreatedAt      time.Time `db:"created_at"`       // Время создания записи
	UpdatedAt      time.Time `db:"updated_at"`       // Время последнего обновления
}

// MeetingWithRank используется для поиска с ранжированием
type MeetingWithRank struct {
	Meeting
	Rank float64 `db:"rank"` // Ранг релевантности для поисковых запросов
}

// SearchQuery параметры поиска встреч
type SearchQuery struct {
	UserID int64  // Telegram ID пользователя
	Query  string // Поисковый запрос
	Limit  int    // Количество результатов для возврата на страницу
	Offset int    // Смещение для пагинации
}

// ListQuery параметры получения списка встреч
type ListQuery struct {
	UserID int64 // Telegram ID пользователя
	Limit  int   // Количество результатов для возврата на страницу
	Offset int   // Смещение для пагинации
}

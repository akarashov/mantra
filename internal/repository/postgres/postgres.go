// package postgres для ортанизации работы с СУБД 
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/akarashov/mantra/internal/config"
	"github.com/akarashov/mantra/internal/repository"
	"github.com/akarashov/mantra/pkg/logger"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Postgres реализует интерфейс к БД
type Postgres struct {
	db     *sqlx.DB
	logger *logger.Logger
}

// New создаёт новое подключение к PostgreSQL и если есть миграции, то выполняет их
func New(cfg config.Database, log *logger.Logger) (*Postgres, error) {
	db, err := sqlx.Connect("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	if cfg.ConnMaxLife != "" {
		if duration, err := time.ParseDuration(cfg.ConnMaxLife); err == nil {
			db.SetConnMaxLifetime(duration)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	err = makeMigraton("file://migrations", cfg.DSN)
	if err != nil {
		return &Postgres{db: db, logger: log}, err
	}
	log.Info("connected to postgres database")
	return &Postgres{
		db:     db,
		logger: log,
	}, nil
}

// makeMigraton выполняет миграцию базы данных, используя файлы миграций из указанной директории
func makeMigraton(pathMigrations string, dataBaseDSN string) error {
	m, err := migrate.New(pathMigrations, dataBaseDSN)
	if err != nil {
		return err
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// Close закрывает подключение к БД
func (p *Postgres) Close() error {
	return p.db.Close()
}

// CreateUser создаёт или обновляет пользователя  UPSERT по Telegram ID
func (p *Postgres) CreateUser(ctx context.Context, telegramID int64, username string) (*repository.User, error) {
	user := &repository.User{
		TelegramID: telegramID,
		Username:   username,
		CreatedAt:  time.Now(),
	}
	err := p.db.GetContext(ctx, user,
		`INSERT INTO users (telegram_id, username, created_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (telegram_id) DO UPDATE 
		 SET username = EXCLUDED.username
		 RETURNING id, telegram_id, username, created_at`,
		telegramID, username, user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return user, nil
}

// GetUserByTelegramID находит пользователя по Telegram ID
func (p *Postgres) GetUserByTelegramID(ctx context.Context, telegramID int64) (*repository.User, error) {
	user := &repository.User{}
	err := p.db.GetContext(ctx, user,
		`SELECT id, telegram_id, username, created_at 
		 FROM users WHERE telegram_id = $1`,
		telegramID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// CreateMeeting сохраняет новую встречу
func (p *Postgres) CreateMeeting(ctx context.Context, meeting *repository.Meeting) (*repository.Meeting, error) {
	err := p.db.GetContext(ctx, meeting,
		`WITH inserted_meeting AS (
    	INSERT INTO meetings 
    	(user_id, telegram_file_id, original_name, duration, transcript, summary)
    	SELECT id, $2, $3, $4, $5, $6
    	FROM users 
    	WHERE users.telegram_id = $1
    	RETURNING *
		)
		SELECT 
    	inserted_meeting.id,
    	$1 AS user_id,
    	inserted_meeting.telegram_file_id,
    	inserted_meeting.original_name,
    	inserted_meeting.duration,
    	inserted_meeting.transcript,
    	inserted_meeting.summary,
    	inserted_meeting.created_at,
    	inserted_meeting.updated_at
		FROM inserted_meeting;`,
		meeting.UserID, meeting.TelegramFileID, meeting.OriginalName,
		meeting.Duration, meeting.Transcript, meeting.Summary)
	if err != nil {
		return nil, fmt.Errorf("insert meeting: %w", err)
	}
	return meeting, nil
}

// GetMeetingByID получает встречу по ID встречи и Telegram ID пользователя
func (r *Postgres) GetMeetingByID(ctx context.Context, meetingID, userID int64) (*repository.Meeting, error) {
	meeting := &repository.Meeting{}
	err := r.db.GetContext(ctx, meeting,
		`SELECT m.id, u.telegram_id as user_id, m.telegram_file_id, 
		m.original_name, m.duration, m.transcript, m.summary,
		m.created_at, m.updated_at FROM meetings m join users u on m.user_id = u.id
		WHERE m.id = $1 AND u.telegram_id = $2`,
		meetingID, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting: %w", err)
	}
	return meeting, nil
}

// GetMeetingsByUser получает список встреч пользователя
func (r *Postgres) GetMeetingsByUser(ctx context.Context, query repository.ListQuery) ([]repository.Meeting, error) {
	var meetings []repository.Meeting
	err := r.db.SelectContext(ctx, &meetings,
		`SELECT m.id, u.telegram_id as user_id, m.telegram_file_id, 
		m.original_name, m.duration, m.transcript, m.summary,
		m.created_at, m.updated_at FROM meetings m join users u on m.user_id = u.id
		WHERE u.telegram_id = $1 ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`,
		query.UserID, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	return meetings, nil
}

// UpdateMeeting обновляет транскрипцию и выжимку встречи
func (r *Postgres) UpdateMeeting(ctx context.Context, meetingID, userID int64, transcript, summary string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE meetings m SET transcript = $1, summary = $2
		 FROM users u 
		 WHERE m.user_id = u.id AND
		 m.id = $3 AND 
		 u.telegram_id = $4`,
		transcript, summary, meetingID, userID)
	if err != nil {
		return fmt.Errorf("update meeting: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("meeting not found or access denied")
	}
	return nil
}

// SearchMeetings выполняет полнотекстовый поиск по встречам
func (r *Postgres) SearchMeetings(ctx context.Context, query repository.SearchQuery) ([]repository.MeetingWithRank, error) {
	var meetings []repository.MeetingWithRank
	safeQuery := strings.ReplaceAll(query.Query, "'", "''")
	err := r.db.SelectContext(ctx, &meetings,
		`SELECT m.id, u.telegram_id as user_id, m.telegram_file_id, m.original_name, 
		 m.duration, m.transcript, m.summary, m.created_at, m.updated_at, 
		 ts_rank(m.transcript_tsvector, plainto_tsquery('russian', $2)) as rank
		 FROM meetings m join users u on m.user_id = u.id
		 WHERE u.telegram_id = $1 
		 AND m.transcript_tsvector @@ plainto_tsquery('russian', $2)
		 ORDER BY rank DESC, created_at DESC
		 LIMIT $3 OFFSET $4`,
		query.UserID, safeQuery, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("search meetings: %w", err)
	}
	return meetings, nil
}

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/akarashov/mantra/internal/repository"
)

type MeetingRepo struct {
	pg *Postgres
}

func NewMeetingRepo(pg *Postgres) *MeetingRepo {
	return &MeetingRepo{pg: pg}
}

// Create Создание встречи
func (r *MeetingRepo) Create(ctx context.Context, meeting *repository.Meeting) (*repository.Meeting, error) {
	if meeting == nil {
		return nil, fmt.Errorf("nil meeting")
	}
	if err := r.pg.db.GetContext(ctx, meeting,
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
		meeting.Duration, meeting.Transcript, meeting.Summary); err != nil {
		return nil, fmt.Errorf("insert meeting: %w", err)
	}
	return meeting, nil
}

// Read получение встречи
func (r *MeetingRepo) Read(ctx context.Context, key repository.MeetingKey) (*repository.Meeting, error) {
	meeting := &repository.Meeting{}
	if err := r.pg.db.GetContext(ctx, meeting,
		`SELECT m.id, u.telegram_id as user_id, m.telegram_file_id, 
		m.original_name, m.duration, m.transcript, m.summary,
		m.created_at, m.updated_at FROM meetings m join users u on m.user_id = u.id
		WHERE m.id = $1 AND u.telegram_id = $2`,
		key.ID, key.UserID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get meeting: %w", err)
	}
	return meeting, nil
}

// Update обновление встречи
func (r *MeetingRepo) Update(ctx context.Context, key repository.MeetingKey, meeting *repository.Meeting) error {
	result, err := r.pg.db.ExecContext(ctx,
		`UPDATE meetings m SET transcript = $1, summary = $2
		 FROM users u 
		 WHERE m.user_id = u.id AND
		 m.id = $3 AND 
		 u.telegram_id = $4`,
		meeting.Transcript, meeting.Summary, key.ID, key.UserID)
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

// List получение списка встреч возвращает лимитированный список
func (r *MeetingRepo) List(ctx context.Context, query repository.ListQuery) ([]repository.Meeting, error) {
	var meetings []repository.Meeting
	if err := r.pg.db.SelectContext(ctx, &meetings,
		`SELECT m.id, u.telegram_id as user_id, m.telegram_file_id, 
		m.original_name, m.duration, m.transcript, m.summary,
		m.created_at, m.updated_at FROM meetings m join users u on m.user_id = u.id
		WHERE u.telegram_id = $1 ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`,
		query.UserID, query.Limit, query.Offset); err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	return meetings, nil
}

// Search полнотекстовый поиск встреч, возвращает упорядоченный список
func (r *MeetingRepo) Search(ctx context.Context, query repository.SearchQuery) ([]repository.MeetingWithRank, error) {
	var meetings []repository.MeetingWithRank
	safeQuery := strings.ReplaceAll(query.Query, "'", "''")
	if err := r.pg.db.SelectContext(ctx, &meetings,
		`SELECT m.id, u.telegram_id as user_id, m.telegram_file_id, m.original_name, 
		 m.duration, m.transcript, m.summary, m.created_at, m.updated_at, 
		 ts_rank(m.transcript_tsvector, plainto_tsquery('russian', $2)) as rank
		 FROM meetings m join users u on m.user_id = u.id
		 WHERE u.telegram_id = $1 
		 AND m.transcript_tsvector @@ plainto_tsquery('russian', $2)
		 ORDER BY rank DESC, created_at DESC
		 LIMIT $3 OFFSET $4`,
		query.UserID, safeQuery, query.Limit, query.Offset); err != nil {
		return nil, fmt.Errorf("search meetings: %w", err)
	}
	return meetings, nil
}

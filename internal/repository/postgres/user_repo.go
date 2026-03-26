package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/akarashov/mantra/internal/repository"
)

// UserRepo реализует UserRepository поверх Postgres
type UserRepo struct {
	pg *Postgres
}

func NewUserRepo(pg *Postgres) *UserRepo {
	return &UserRepo{pg: pg}
}

// Create Апсерт пользователя
func (r *UserRepo) Create(ctx context.Context, user *repository.User) (*repository.User, error) {
	if user == nil {
		return nil, fmt.Errorf("nil user")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if err := r.pg.db.GetContext(ctx, user,
		`INSERT INTO users (telegram_id, username, created_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (telegram_id) DO UPDATE 
		 SET username = EXCLUDED.username
		 RETURNING id, telegram_id, username, created_at`,
		user.TelegramID, user.Username, user.CreatedAt); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return user, nil
}

// Read получение пользователя
func (r *UserRepo) Read(ctx context.Context, telegramID int64) (*repository.User, error) {
	user := &repository.User{}
	if err := r.pg.db.GetContext(ctx, user,
		`SELECT id, telegram_id, username, created_at FROM users WHERE telegram_id = $1`,
		telegramID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// Update обновление пользователя
func (r *UserRepo) Update(ctx context.Context, telegramID int64, user *repository.User) error {
	_, err := r.pg.db.ExecContext(ctx,
		`UPDATE users SET username = $1 WHERE telegram_id = $2`,
		user.Username, telegramID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

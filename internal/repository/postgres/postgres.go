// package postgres для ортанизации работы с СУБД
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/akarashov/mantra/internal/config"
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

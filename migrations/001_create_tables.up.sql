-- +goose Up
-- +goose StatementBegin

-- Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Таблица встреч
CREATE TABLE IF NOT EXISTS meetings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    telegram_file_id VARCHAR(255) NOT NULL,
    original_name VARCHAR(255),
    duration INTEGER DEFAULT 0,
    transcript TEXT,
    transcript_tsvector TSVECTOR,
    summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    constraint valid_timeline check (updated_at >= created_at)
);

-- Создаем функцию для обновления tsvector полнотекстового поиска
CREATE OR REPLACE
FUNCTION meetings_transcript_tsvector_update() 
RETURNS TRIGGER AS $$
BEGIN
    NEW.transcript_tsvector = to_tsvector('russian', COALESCE(NEW.transcript, ''));
    RETURN NEW;
END;

$$ LANGUAGE 'plpgsql';

-- Создаем триггер для автоматического обновления tsvector при вставке или обновлении записи в таблице meetings
CREATE TRIGGER meetings_transcript_tsvector_trigger
    BEFORE
update OR insert
    ON
    meetings
    FOR EACH ROW
    EXECUTE procedure meetings_transcript_tsvector_update();

-- Индексы
CREATE INDEX IF NOT EXISTS idx_meetings_user_id ON meetings(user_id);
CREATE INDEX IF NOT EXISTS idx_meetings_created_at ON meetings(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_meetings_user_created ON meetings(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_meetings_tsvector ON meetings USING GIN(transcript_tsvector);

-- Функция для авто-обновления updated_at
CREATE OR REPLACE
FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;

$$ LANGUAGE 'plpgsql';

-- Создаем триггер для обновления updated_at при каждом обновлении записи в таблице meetings
CREATE TRIGGER trg_update_meetings_updated_at
    BEFORE
    UPDATE
    ON
    meetings
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- +goose StatementEnd

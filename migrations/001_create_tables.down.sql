-- +goose Down
-- +goose StatementBegin

-- Удаляем триггер и функцию
DROP TRIGGER IF EXISTS trg_update_meetings_updated_at ON meetings;
DROP TRIGGER IF EXISTS meetings_transcript_tsvector_trigger ON meetings;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS meetings_transcript_tsvector_update();

-- Удаляем индексы
DROP INDEX IF EXISTS idx_meetings_transcript_tsvector;
DROP INDEX IF EXISTS idx_meetings_user_id;
DROP INDEX IF EXISTS idx_meetings_created_at;
DROP INDEX IF EXISTS idx_meetings_user_created;

-- Удаляем таблицы (CASCADE удалит зависимые объекты)
DROP TABLE IF EXISTS meetings;
DROP TABLE IF EXISTS users;

-- +goose StatementEnd
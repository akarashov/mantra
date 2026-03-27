// Package utils содержит вспомогательные функции для обработки строк, формата времени, работы с файлами и другими утилитами, используемыми в проекте.
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DateTimeFormat = "2006-01-02T15:04:05Z07:00"

// SafeString возвращает безопасную строку для отображения в Telegram, экранируя спецсимволы и заменяя пустые строки на "Пользователь"
func SafeString(s string) string {
	if s == "" {
		return "Пользователь"
	}
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}

// TruncateString обрезает строку до указанной длины с добавлением многоточия
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ParseInt64 парсит строку в int64
func ParseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// FormatDuration форматирует длительность в секундах в человекочитаемый вид
func FormatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d сек", seconds)
	}
	if seconds < 3600 {
		mins := seconds / 60
		secs := seconds % 60
		if secs == 0 {
			return fmt.Sprintf("%d мин", mins)
		}
		return fmt.Sprintf("%d мин %d сек", mins, secs)
	}
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	if mins == 0 {
		return fmt.Sprintf("%d ч", hours)
	}
	return fmt.Sprintf("%d ч %d мин", hours, mins)
}

// IsAudioFile проверяет, является ли файл аудио по расширению
func IsAudioFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	audioExts := map[string]bool{
		".mp3":  true,
		".wav":  true,
		".ogg":  true,
		".m4a":  true,
		".flac": true,
		".aac":  true,
		".opus": true,
	}
	return audioExts[ext]
}

// HighlightKeywords подсвечивает ключевые слова в тексте
func HighlightKeywords(text, keywords string, previewLen int) string {
	preview := text
	if len(preview) > previewLen {
		preview = preview[:previewLen]
		if lastSpace := strings.LastIndex(preview, " "); lastSpace > previewLen/2 {
			preview = preview[:lastSpace]
		}
		preview += "..."
	}
	words := strings.FieldsSeq(keywords)
	for word := range words {
		if len(word) >= 3 {
			escaped := regexp.QuoteMeta(word)
			re := regexp.MustCompile(`(?i)\b` + escaped + `\b`)
			preview = re.ReplaceAllString(preview, "**$0**")
		}
	}
	return preview
}

// Getenv возвращает значение переменной окружения или значение по умолчанию
func Getenv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// GetContentTypeFromExtension возвращает MIME-type по расширению файла
func GetContentTypeFromExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".opus":
		return "audio/ogg;codecs=opus"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a", ".mp4":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	default:
		return "application/octet-stream"
	}
}

// GetExtensionFromContentType возвращает расширение по MIME-type
func GetExtensionFromContentType(contentType string) string {
	mainType := strings.Split(contentType, ";")[0]
	switch mainType {
	case "audio/ogg":
		return "ogg"
	case "audio/mpeg":
		return "mp3"
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "audio/mp4", "audio/aac":
		return "m4a"
	case "audio/flac":
		return "flac"
	default:
		return "bin"
	}
}

// FormatDurationMs форматирует длительность в миллисекундах
func FormatDurationMs(ms int64) string {
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%d сек", seconds)
	}
	if seconds < 3600 {
		mins := seconds / 60
		secs := seconds % 60
		if secs == 0 {
			return fmt.Sprintf("%d мин", mins)
		}
		return fmt.Sprintf("%d мин %d сек", mins, secs)
	}
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	if mins == 0 {
		return fmt.Sprintf("%d ч", hours)
	}
	return fmt.Sprintf("%d ч %d мин", hours, mins)
}

// GenerateShortID создаёт короткий уникальный идентификатор
func GenerateShortID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano()%0xFFFFFF)
}

// FormatDateTime форматирует время в строку по заданному формату
func FormatDateTime(t time.Time) string {
	return t.Format(DateTimeFormat)
}

package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/akarashov/mantra/internal/services/speech"
	"github.com/akarashov/mantra/pkg/utils"

	"gopkg.in/telebot.v3"
)

// handleVoice - обработка голосового сообщения (in-memory) Telegram voice всегда в OGG/Opus
func (h *Handler) handleVoice(c telebot.Context) error {
	voice := c.Message().Voice
	if voice == nil {
		return c.Reply("Не удалось получить голосовое сообщение")
	}
	return h.processAudioInMemory(c, voice.FileID, int64(voice.Duration*1000), "audio/ogg")
}

// handleAudio - обработка аудиофайла (in-memory)
func (h *Handler) handleAudio(c telebot.Context) error {
	fileID, durationMs, contentType, err := h.extractAudioInfo(c)
	if err != nil {
		return c.Reply(err.Error())
	}
	return h.processAudioInMemory(c, fileID, durationMs, contentType)
}

// extractAudioInfo извлекает информацию о аудиофайле из сообщения
func (h *Handler) extractAudioInfo(c telebot.Context) (fileID string, durationMs int64, contentType string, err error) {
	switch {
	case c.Message().Audio != nil:
		audio := c.Message().Audio
		fileID = audio.FileID
		durationMs = int64(audio.Duration * 1000)
		if audio.MIME != "" {
			contentType = audio.MIME
		} else {
			contentType = "audio/mpeg"
		}
		if audio.FileName == "" {
			audio.FileName = fmt.Sprintf("audio_%d.mp3", time.Now().Unix())
		}
	case c.Message().Document != nil:
		doc := c.Message().Document
		if !utils.IsAudioFile(doc.FileName) {
			err = fmt.Errorf("Поддерживаются только аудиофайлы (mp3, wav, ogg, m4a)")
			return
		}
		fileID = doc.FileID
		durationMs = 0
		if doc.MIME != "" {
			contentType = doc.MIME
		} else {
			contentType = utils.GetContentTypeFromExtension(doc.FileName)
		}
	default:
		err = fmt.Errorf("Не распознан тип аудиофайла")
	}
	return
}

// processAudioInMemory - основная логика: скачиваем в память → обрабатываем → сохраняем
func (h *Handler) processAudioInMemory(c telebot.Context, fileID string, durationMs int64, contentType string) error {
	userID := c.Sender().ID
	processingMsg, err := h.sendProcessingMessage(c)
	if err != nil {
		return err
	}
	audioBuffer, err := h.downloadAudioToMemory(fileID, userID, contentType)
	if err != nil {
		h.bot.Edit(processingMsg, "Ошибка загрузки файла из Telegram")
		return nil
	}
	result, err := h.processAudioWithService(userID, audioBuffer, contentType, durationMs, fileID)
	if err != nil {
		h.bot.Edit(processingMsg, fmt.Sprintf("Ошибка обработки: %v", err))
		return nil
	}
	return h.sendResultToUser(processingMsg, result)
}

// sendProcessingMessage отправляет сообщение о начале обработки
func (h *Handler) sendProcessingMessage(c telebot.Context) (*telebot.Message, error) {
	processingMsg, err := c.Bot().Send(c.Chat(), "Ом Бхур Бхувах Сваха\nТат Савитур Вареньям\nБхарго Девасья Дхимахи\nДхийо Йо Нах Прачодайят...")
	if err != nil {
		h.log.Error("failed to send processing message", "error", err)
	}
	return processingMsg, err
}

// downloadAudioToMemory скачивает аудио в память и логирует
func (h *Handler) downloadAudioToMemory(fileID string, userID int64, contentType string) (*bytes.Buffer, error) {
	audioBuffer, err := h.downloadTelegramFileToMemory(fileID)
	if err != nil {
		h.log.Error("failed to download file to memory", "file_id", fileID, "error", err)
		return nil, err
	}
	h.log.Debug("file downloaded to memory",
		"user_id", userID,
		"file_id", fileID,
		"buffer_size", audioBuffer.Len(),
		"content_type", contentType)
	return audioBuffer, nil
}

// processAudioWithService обрабатывает аудио через speech сервис
func (h *Handler) processAudioWithService(userID int64, audioBuffer *bytes.Buffer, contentType string, durationMs int64, fileID string) (*speech.ProcessResult, error) {
	procCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := h.speechService.ProcessMeeting(procCtx, &speech.ProcessInput{
		UserID:         userID,
		AudioData:      bytes.NewReader(audioBuffer.Bytes()),
		FileName:       fmt.Sprintf("meeting_%s.%s", utils.GenerateShortID(), utils.GetExtensionFromContentType(contentType)),
		ContentType:    contentType,
		DurationMs:     durationMs,
		TelegramFileID: fileID,
	})
	if err != nil {
		h.log.Error("speech service error", "user_id", userID, "error", err)
		return nil, err
	}
	return result, nil
}

// sendResultToUser отправляет результат обработки пользователю
func (h *Handler) sendResultToUser(processingMsg *telebot.Message, result *speech.ProcessResult) error {
	summary := utils.TruncateString(result.Summary, 1000)
	response := fmt.Sprintf("Обработка завершена!\n\n"+
		"id встречи: %d\n"+
		"Длительность: %s\n"+
		"Дата: %s\n\n"+
		"*Краткая выжимка:*\n"+
		"%s\n\n"+
		"Используйте:\n"+
		"  /get %d — полная транскрипция\n"+
		"  /chat <вопрос> — спросить ИИ о деталях",
		result.MeetingID,
		utils.FormatDurationMs(result.DurationMs),
		result.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		summary,
		result.MeetingID)
	h.bot.Edit(processingMsg, response, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
	return nil
}

// downloadTelegramFileToMemory скачивает файл из Telegram в bytes.Buffer (полностью в памяти)
func (h *Handler) downloadTelegramFileToMemory(fileID string) (*bytes.Buffer, error) {
	fileInfo, err := h.bot.FileByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("get file info: %w", err)
	}
	initialCap := 128 * 1024
	if fileInfo.FileSize > 0 && fileInfo.FileSize < 20*1024*1024 { // макс 20MB для Telegram
		initialCap = int(fileInfo.FileSize)
	}
	buffer := bytes.NewBuffer(make([]byte, 0, initialCap))
	fileURL := h.bot.URL + "/file/bot" + h.bot.Token + "/" + fileInfo.FilePath
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copy to buffer: %w", err)
	}
	return buffer, nil
}

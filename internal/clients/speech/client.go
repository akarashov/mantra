// Package speech предоставляет gRPC клиент для взаимодействия с SmartSpeech API
package speech

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/akarashov/mantra/internal/clients/auth"
	"github.com/akarashov/mantra/pkg/grpc/recognition"
	"github.com/akarashov/mantra/pkg/grpc/storage"
	"github.com/akarashov/mantra/pkg/grpc/task"
	"github.com/akarashov/mantra/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Client предоставляет gRPC клиент SmartSpeech
type Client struct {
	conn         *grpc.ClientConn
	recognition  recognition.SmartSpeechClient
	storage      storage.SmartSpeechClient
	task         task.SmartSpeechClient
	model        string
	pollInterval time.Duration
	log          *logger.Logger
	auth         *auth.AuthClient
}

// NewClient создаёт новый gRPC клиент SmartSpeech
func NewClient(
	endpoint, clientID, clientSecret, scope, authURL, modelURI, certPath string,
	pollInterval time.Duration,
	log *logger.Logger,
	insecure bool,
) (*Client, error) {
	var opts []grpc.DialOption
	if insecure {
		log.Debug("creating new grpc client without TLS (insecure)", "endpoint", endpoint)
		opts = append(opts, grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	} else {
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			return nil, fmt.Errorf("read cert file: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(certPEM) {
			return nil, fmt.Errorf("failed to add cert to pool")
		}
		log.Debug("creating new grpc client with TLS", "endpoint", endpoint)
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(certPool, "")))
	}
	log.Debug("creating new grpc client", "insecure", insecure)
	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial grpc: %w", err)
	}
	authClient, err := auth.NewAuthClient(clientID, clientSecret, scope, authURL, certPath, log)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create auth client: %w", err)
	}
	client := &Client{
		conn:         conn,
		recognition:  recognition.NewSmartSpeechClient(conn),
		storage:      storage.NewSmartSpeechClient(conn),
		task:         task.NewSmartSpeechClient(conn),
		model:        modelURI,
		pollInterval: pollInterval,
		log:          log,
		auth:         authClient,
	}
	return client, nil
}

// Close закрывает gRPC соединение
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// AsyncTranscribe выполняет асинхронную транскрибацию аудио
func (c *Client) AsyncTranscribe(ctx context.Context, audio io.Reader, fileName string, contentType string, durationMs int64) (string, error) {
	c.log.Debug("starting async transcription",
		"file", fileName,
		"content_type", contentType)
	token, err := c.auth.GetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx = metadata.NewOutgoingContext(ctx, md)
	requestFileID, err := c.uploadFile(ctx, audio)
	if err != nil {
		return "", fmt.Errorf("grpc upload file: %w", err)
	}
	taskResp, err := c.recognition.AsyncRecognize(ctx, &recognition.AsyncRecognizeRequest{
		Options: &recognition.RecognitionOptions{
			AudioEncoding: audioEncodingFromContentType(contentType),
			Language:      "ru-RU",
			Model:         c.model, // For async, probably no partial
		},
		RequestFileId: requestFileID,
	})
	if err != nil {
		return "", fmt.Errorf("create async recognition task: %w", err)
	}
	taskID := taskResp.Id
	c.log.Debug("task created", "task_id", taskID)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		taskStatus, err := c.task.GetTask(ctx, &task.GetTaskRequest{TaskId: taskID})
		if err != nil {
			return "", fmt.Errorf("get task status: %w", err)
		}
		switch taskStatus.Status {
		case task.Task_DONE:
			responseFileID := taskStatus.GetResponseFileId()
			if responseFileID == "" {
				return "", fmt.Errorf("no response file id in done task")
			}
			result, err := c.downloadResult(ctx, responseFileID)
			if err != nil {
				return "", fmt.Errorf("download result: %w", err)
			}
			c.log.Debug("task completed, result downloaded", "task_id", taskID, "result", result)
			return result, nil
		case task.Task_ERROR:
			return "", fmt.Errorf("task error: %s", taskStatus.GetError())
		case task.Task_CANCELED:
			return "", fmt.Errorf("task canceled")
		case task.Task_NEW, task.Task_RUNNING:
			time.Sleep(c.pollInterval)
		default:
			return "", fmt.Errorf("unknown task status: %v", taskStatus.Status)
		}
	}
}

// uploadFile загружает файл на сервер и возвращает request_file_id
func (c *Client) uploadFile(ctx context.Context, audio io.Reader) (string, error) {
	stream, err := c.storage.Upload(ctx)
	if err != nil {
		return "", fmt.Errorf("create upload stream: %w", err)
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := audio.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if sendErr := stream.Send(&storage.UploadRequest{FileChunk: chunk}); sendErr != nil {
				return "", fmt.Errorf("send chunk: %w", sendErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read audio: %w", err)
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", fmt.Errorf("close upload stream: %w", err)
	}
	return resp.RequestFileId, nil
}

// downloadResult скачивает результат по response_file_id и возвращает как строку
func (c *Client) downloadResult(ctx context.Context, responseFileID string) (string, error) {
	stream, err := c.storage.Download(ctx, &storage.DownloadRequest{ResponseFileId: responseFileID})
	if err != nil {
		return "", fmt.Errorf("create download stream: %w", err)
	}
	var result []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("receive chunk: %w", err)
		}
		result = append(result, resp.FileChunk...)
	}
	nt, err := resultPase(string(result))
	if err != nil {
		return "", fmt.Errorf("parse transcription result: %w", err)
	}
	return string(nt), nil
}

// audioEncodingFromContentType сопоставляет MIME-type с enum RecognitionOptions_AudioEncoding
//
//	Допустимые форматы аудио:
//	 https://developers.sber.ru/docs/ru/salutespeech/guides/recognition/encodings
func audioEncodingFromContentType(contentType string) recognition.RecognitionOptions_AudioEncoding {
	switch contentType {
	case "audio/ogg", "audio/ogg;codecs=opus":
		return recognition.RecognitionOptions_OPUS
	case "audio/x-pcm;bit=16;rate=XXX":
		return recognition.RecognitionOptions_PCM_S16LE
	case "audio/mpeg", "audio/mp3":
		return recognition.RecognitionOptions_MP3
	case "audio/flac":
		return recognition.RecognitionOptions_FLAC
	case "audio/pcma;rate=XXX":
		return recognition.RecognitionOptions_ALAW
	case "audio/pcmu;rate=XXX":
		return recognition.RecognitionOptions_MULAW
	default:
		return recognition.RecognitionOptions_AUDIO_ENCODING_UNSPECIFIED
	}
}

// resultPase парсит JSON результат транскрибации и извлекает нормализованный текст
func resultPase(content string) (string, error) {
	var responses []TranscriptionResponse
	err := json.Unmarshal([]byte(content), &responses)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return "", fmt.Errorf("parse transcription result: %w", err)
	}
	return responses[0].Results[0].NormalizedText, nil
}

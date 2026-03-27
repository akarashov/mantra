// Package auth предоставляет клиент для OAuth аутентификации
package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/akarashov/mantra/pkg/logger"
	"github.com/google/uuid"
)

// AuthClient предоставляет OAuth клиент с автоматической ротацией токенов
type AuthClient struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	scope        string
	authURL      string
	certPath     string
	log          *logger.Logger
	mu           sync.RWMutex
	accessToken  string
	tokenExpiry  time.Time
}

// NewAuthClient создаёт новый OAuth клиент с сертами (хотелось бы Минцифра)
func NewAuthClient(clientID, clientSecret, scope, authURL, certPath string, log *logger.Logger) (*AuthClient, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	if certPath != "" {
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			return nil, fmt.Errorf("read cert file: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(certPEM) {
			return nil, fmt.Errorf("failed to add cert to pool")
		}
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
		}
	}
	return &AuthClient{
		httpClient:   httpClient,
		clientID:     clientID,
		clientSecret: clientSecret,
		scope:        scope,
		authURL:      authURL,
		certPath:     certPath,
		log:          log,
	}, nil
}

// EnsureAuth проверяет и обновляет access token с double-checked locking
func (c *AuthClient) EnsureAuth(ctx context.Context) error {
	c.log.Debug("check auth")
	c.mu.RLock()
	valid := c.accessToken != "" && time.Now().Before(c.tokenExpiry)
	c.mu.RUnlock()
	if valid {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return nil
	}
	token, expiresAt, err := c.requestToken(ctx)
	if err != nil {
		c.log.Debug("fail token request")
		return err
	}
	c.accessToken = token
	c.tokenExpiry = expiresAt
	return nil
}

// GetToken возвращает текущий access token, обновляя если необходимо
func (c *AuthClient) GetToken(ctx context.Context) (string, error) {
	if err := c.EnsureAuth(ctx); err != nil {
		return "", err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken, nil
}

// Do выполняет HTTP запрос с авторизацией
func (c *AuthClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	token, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.httpClient.Do(req)
}

// requestToken запрашивает новый токен доступа
func (c *AuthClient) requestToken(ctx context.Context) (string, time.Time, error) {
	authKey := base64.StdEncoding.EncodeToString([]byte(c.clientID + ":" + c.clientSecret))
	rqUID := uuid.New().String()
	data := "scope=" + c.scope
	req, err := http.NewRequestWithContext(ctx, "POST", c.authURL, strings.NewReader(data))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", rqUID)
	req.Header.Set("Authorization", "Basic "+authKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("auth failed [%d]: %s", resp.StatusCode, string(body))
	}
	var authResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(time.Duration(authResp.ExpiresIn-60) * time.Second)
	return authResp.AccessToken, expiresAt, nil
}

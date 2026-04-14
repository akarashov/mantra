package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/akarashov/mantra/internal/config"
	"github.com/akarashov/mantra/pkg/logger"
)

// Client реализует минимальную интеграцию с HashiCorp Vault
type Client struct {
	cfg      config.Vault
	http     *http.Client
	log      *logger.Logger
	mu       sync.RWMutex
	token    string
	tokenTTL time.Duration
	tokenExp time.Time
}

// New создаёт клиента Vault
func New(cfg config.Vault, log *logger.Logger) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 15 * time.Second},
		log:  log.With("component", "vault_client"),
	}
}

// AuthenticateAppRole логинится в Vault через AppRole.
// Если в конфиге указан AppRoleSecretWrapped=true, предполагается, что
// AppRoleSecretID содержит wrapping token который нужно развернуть перед использованием.
func (c *Client) AuthenticateAppRole(ctx context.Context) error {
	c.log.Debug("authenticate via approle")
	secretID := c.cfg.AppRoleSecretID
	if c.cfg.AppRoleSecretWrapped && secretID != "" {
		unwrapped, err := c.unwrapWrappingToken(ctx, secretID)
		if err != nil {
			return fmt.Errorf("unwrap secret_id: %w", err)
		}
		secretID = unwrapped
	}
	if c.cfg.AppRoleRoleID == "" || secretID == "" {
		return errors.New("approle role_id or secret_id is empty")
	}
	body := fmt.Sprintf(`{"role_id":"%s","secret_id":"%s"}`, c.cfg.AppRoleRoleID, secretID)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.cfg.Addr, "/")+"/v1/auth/approle/login", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approle login failed: %s", string(b))
	}
	var out struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	c.mu.Lock()
	c.token = out.Auth.ClientToken
	c.tokenTTL = time.Duration(out.Auth.LeaseDuration) * time.Second
	c.tokenExp = time.Now().Add(c.tokenTTL)
	c.mu.Unlock()
	c.log.Info("obtained vault token", "ttl_s", out.Auth.LeaseDuration)
	// If using wrapped secret_id, rotate it after first use so wrapped token is single-use
	if c.cfg.AppRoleSecretWrapped {
		go func() {
			// best-effort rotation in background
			if err := c.RotateWrappedSecret(context.Background()); err != nil {
				c.log.Info("failed rotate wrapped secret after use", "err", err)
			}
		}()
	}
	return nil
}

// RotateWrappedSecret создает новый wrapped secret_id для AppRole и сохраняет wrapping token в памяти.
func (c *Client) RotateWrappedSecret(ctx context.Context) error {
	if c.cfg.AppRoleRoleName == "" {
		return fmt.Errorf("approle role name is empty")
	}
	// choose token to perform creation: prefer management token from config
	mgmtToken := c.cfg.Token
	if mgmtToken == "" {
		c.mu.RLock()
		mgmtToken = c.token
		c.mu.RUnlock()
	}
	if mgmtToken == "" {
		return fmt.Errorf("no token available to create secret-id")
	}
	url := strings.TrimRight(c.cfg.Addr, "/") + "/v1/auth/approle/role/" + c.cfg.AppRoleRoleName + "/secret-id"
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Vault-Token", mgmtToken)
	if c.cfg.WrapTTL != "" {
		req.Header.Set("X-Vault-Wrap-TTL", c.cfg.WrapTTL)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create wrapped secret-id failed: %s", string(b))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	// expecting wrap_info.token
	var wrapToken string
	if wi, ok := out["wrap_info"].(map[string]any); ok {
		if t, ok := wi["token"].(string); ok {
			wrapToken = t
		}
	}
	if wrapToken == "" {
		// try nested structure or fail
		raw, _ := json.Marshal(out)
		return fmt.Errorf("no wrap_info.token in response: %s", string(raw))
	}
	// save new wrapped token into config and optionally to file
	c.mu.Lock()
	c.cfg.AppRoleSecretID = wrapToken
	c.mu.Unlock()
	if c.cfg.WrappedTokenFile != "" {
		// ensure dir exists
		dir := filepath.Dir(c.cfg.WrappedTokenFile)
		if dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0700)
		}
		// write token with 0600
		if err := os.WriteFile(c.cfg.WrappedTokenFile, []byte(wrapToken), 0600); err != nil {
			c.log.Info("failed to write wrapped token to file", "file", c.cfg.WrappedTokenFile, "err", err)
		} else {
			c.log.Info("wrote wrapped token to file", "file", c.cfg.WrappedTokenFile)
		}
	}
	c.log.Info("rotated wrapped secret-id", "wrap_token_len", len(wrapToken))
	return nil
}

// StartAutoRotate запускает фоновый тикер для периодической ротации wrapped-token
func (c *Client) StartAutoRotate(ctx context.Context) {
	if c.cfg.RotateInterval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(c.cfg.RotateInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.RotateWrappedSecret(ctx); err != nil {
					c.log.Info("auto rotate wrapped secret failed", "err", err)
				}
			}
		}
	}()
}

// unwrapWrappingToken развертывает wrapping token и возвращает полезную нагрузку как строку.
func (c *Client) unwrapWrappingToken(ctx context.Context, wrappingToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.cfg.Addr, "/")+"/v1/sys/wrapping/unwrap", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", wrappingToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unwrap failed: %s", string(b))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	// Попробуем найти секрет_id в поле data.secret_id или data.value
	if data, ok := out["data"].(map[string]any); ok {
		if sid, ok := data["secret_id"].(string); ok && sid != "" {
			return sid, nil
		}
		if val, ok := data["value"].(string); ok && val != "" {
			return val, nil
		}
	}
	// В неожиданных случаях вернём весь JSON как строку
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

// GetDatabaseCredentials запрашивает динамические учетные данные для роли базы данных
func (c *Client) GetDatabaseCredentials(ctx context.Context, role string) (username, password string, ttl time.Duration, err error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()
	if token == "" {
		if err := c.AuthenticateAppRole(ctx); err != nil {
			return "", "", 0, err
		}
		c.mu.RLock()
		token = c.token
		c.mu.RUnlock()
	}
	url := strings.TrimRight(c.cfg.Addr, "/") + "/v1/database/creds/" + role
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("X-Vault-Token", token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("db creds request failed: %s", string(b))
	}
	var out struct {
		Data struct {
			Username string `json:"username"`
			Password string `json:"password"`
			TTL      int    `json:"ttl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", 0, err
	}
	return out.Data.Username, out.Data.Password, time.Duration(out.Data.TTL) * time.Second, nil
}

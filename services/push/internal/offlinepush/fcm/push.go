package fcm

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lisuhang11/SuIM/services/push/internal/config"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

const (
	defaultScope    = "https://www.googleapis.com/auth/firebase.messaging"
	defaultTokenURL = "https://oauth2.googleapis.com/token"
	sendURLTemplate = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
)

type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// Client 用 FCM HTTP v1 按设备 token 逐条发送通知。
type Client struct {
	cfg       *config.Push
	tokens    options.Tokens
	http      *http.Client
	account   serviceAccount
	key       *rsa.PrivateKey
	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

func NewClient(cfg *config.Push, tokens options.Tokens) (*Client, error) {
	if cfg.FCM.CredentialsFile == "" {
		return nil, fmt.Errorf("fcm credentialsFile is required")
	}
	raw, err := os.ReadFile(cfg.FCM.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read fcm credentials: %w", err)
	}
	var account serviceAccount
	if err := json.Unmarshal(raw, &account); err != nil {
		return nil, fmt.Errorf("decode fcm credentials: %w", err)
	}
	if account.ProjectID == "" || account.ClientEmail == "" || account.PrivateKey == "" {
		return nil, fmt.Errorf("fcm credentials missing project_id, client_email or private_key")
	}
	block, _ := pem.Decode([]byte(account.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("fcm private key is not pem")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse fcm private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("fcm private key is not rsa")
	}
	if account.TokenURI == "" {
		account.TokenURI = defaultTokenURL
	}
	return &Client{cfg: cfg, tokens: tokens, http: &http.Client{Timeout: 5 * time.Second}, account: account, key: key}, nil
}

// Push 给用户的每台设备发送一条通知。开启角标时先累加 Redis 计数。
func (c *Client) Push(ctx context.Context, userIDs []string, title, content string, opts *options.Opts) error {
	if len(userIDs) == 0 {
		return nil
	}
	if opts != nil && opts.IOSBadgeCount {
		for _, userID := range userIDs {
			if _, err := c.tokens.IncrBadge(ctx, userID, 1); err != nil {
				slog.Warn("incr fcm badge", "err", err, "userID", userID)
			}
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, userID := range userIDs {
		deviceTokens, err := c.tokens.FCMTokens(ctx, userID)
		if err != nil {
			slog.Warn("read fcm token", "err", err, "userID", userID)
			continue
		}
		var badge int64
		if opts != nil && opts.IOSBadgeCount {
			badge, _ = c.tokens.Badge(ctx, userID)
		}
		for _, deviceToken := range deviceTokens {
			wg.Add(1)
			go func(deviceToken string) {
				defer wg.Done()
				if err := c.send(ctx, deviceToken, title, content, badge, opts); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
			}(deviceToken)
		}
	}
	wg.Wait()
	return firstErr
}

func (c *Client) send(ctx context.Context, deviceToken, title, content string, badge int64, opts *options.Opts) error {
	access, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	aps := map[string]any{"sound": "default"}
	if opts != nil && opts.IOSPushSound != "" {
		aps["sound"] = opts.IOSPushSound
	}
	if badge > 0 {
		aps["badge"] = badge
	}
	data := map[string]string{}
	if opts != nil {
		if opts.Ex != "" {
			data["ex"] = opts.Ex
		}
		if opts.Signal != nil && opts.Signal.ClientMsgID != "" {
			data["clientMsgID"] = opts.Signal.ClientMsgID
		}
	}
	message := map[string]any{
		"token":        deviceToken,
		"notification": map[string]string{"title": title, "body": content},
		"apns":         map[string]any{"payload": map[string]any{"aps": aps}},
	}
	if len(data) > 0 {
		message["data"] = data
	}
	body, err := json.Marshal(map[string]any{"message": message})
	if err != nil {
		return fmt.Errorf("encode fcm message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf(sendURLTemplate, c.account.ProjectID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("fcm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fcm post: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read fcm response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fcm status %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != "" && time.Now().Add(time.Minute).Before(c.expiresAt) {
		return c.cached, nil
	}
	now := time.Now()
	claims := map[string]any{
		"iss":   c.account.ClientEmail,
		"scope": defaultScope,
		"aud":   c.account.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign fcm jwt: %w", err)
	}
	jwt := signingInput + "." + enc.EncodeToString(signature)
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", jwt)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.account.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("fcm token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fcm token post: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read fcm token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fcm token status %d: %s", resp.StatusCode, string(raw))
	}
	var result tokenResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode fcm token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("fcm token response empty")
	}
	c.cached = result.AccessToken
	c.expiresAt = now.Add(time.Duration(result.ExpiresIn) * time.Second)
	return c.cached, nil
}

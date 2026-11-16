package getui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/lisuhang11/SuIM/services/push/internal/config"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

const tokenTTL = 23 * time.Hour

// Client 用个推 REST v2 下发通知，alias 直接使用 userID。
type Client struct {
	cfg    *config.Push
	tokens options.Tokens
	http   *http.Client
}

func NewClient(cfg *config.Push, tokens options.Tokens) *Client {
	return &Client{cfg: cfg, tokens: tokens, http: &http.Client{Timeout: 5 * time.Second}}
}

// Push 给每个用户单独下发一条通知，并发等待全部结果。
func (c *Client) Push(ctx context.Context, userIDs []string, title, content string, opts *options.Opts) error {
	if len(userIDs) == 0 {
		return nil
	}
	token, err := c.token(ctx)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	errs := make([]error, len(userIDs))
	for i, userID := range userIDs {
		wg.Add(1)
		go func(i int, userID string) {
			defer wg.Done()
			errs[i] = c.single(ctx, token, userID, title, content, opts)
		}(i, userID)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) token(ctx context.Context) (string, error) {
	cached, err := c.tokens.GetuiToken(ctx)
	if err == nil && cached != "" {
		return cached, nil
	}
	if err != nil {
		slog.Warn("read getui token", "err", err)
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sign := sha256.Sum256([]byte(c.cfg.GeTui.AppKey + ts + c.cfg.GeTui.MasterSecret))
	body := map[string]string{
		"appkey":    c.cfg.GeTui.AppKey,
		"timestamp": ts,
		"sign":      hex.EncodeToString(sign[:]),
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := c.post(ctx, c.cfg.GeTui.PushURL+"/auth", "", body, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", fmt.Errorf("getui auth returned empty token")
	}
	if err := c.tokens.SetGetuiToken(ctx, resp.Token, tokenTTL); err != nil {
		slog.Warn("cache getui token", "err", err)
	}
	return resp.Token, nil
}

func (c *Client) single(ctx context.Context, token, userID, title, content string, opts *options.Opts) error {
	payload := map[string]string{}
	if opts != nil {
		if opts.Ex != "" {
			payload["ex"] = opts.Ex
		}
		if opts.Signal != nil && opts.Signal.ClientMsgID != "" {
			payload["clientMsgID"] = opts.Signal.ClientMsgID
		}
	}
	notification := map[string]any{
		"title":     title,
		"body":      content,
		"clickType": "openurl",
		"url":       c.cfg.GeTui.Intent,
		"payload":   payload,
	}
	pushMessage := map[string]any{"notification": notification}
	if c.cfg.GeTui.ChannelID != "" {
		pushMessage["channel"] = map[string]string{
			"channelId":   c.cfg.GeTui.ChannelID,
			"channelName": c.cfg.GeTui.ChannelName,
		}
	}
	body := map[string]any{
		"requestId":   strconv.FormatInt(time.Now().UnixNano(), 10),
		"appkey":      c.cfg.GeTui.AppKey,
		"audience":    map[string]any{"alias": []string{userID}},
		"pushMessage": pushMessage,
	}
	return c.post(ctx, c.cfg.GeTui.PushURL+"/push/single/alias", token, body, nil)
}

func (c *Client) post(ctx context.Context, url, token string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode getui request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("getui request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	if token != "" {
		req.Header.Set("token", token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("getui post: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read getui response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("getui status %d: %s", resp.StatusCode, string(raw))
	}
	var common struct {
		Code any    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(raw, &common)
	if common.Code != nil {
		code := fmt.Sprint(common.Code)
		if code != "" && code != "0" {
			return fmt.Errorf("getui code %s: %s", code, common.Msg)
		}
	}
	if output != nil {
		if err := json.Unmarshal(raw, output); err != nil {
			return fmt.Errorf("decode getui response: %w", err)
		}
	}
	return nil
}

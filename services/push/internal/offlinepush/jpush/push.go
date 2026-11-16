package jpush

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lisuhang11/SuIM/services/push/internal/config"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

const defaultPushURL = "https://api.jpush.cn/v3/push"

// Client 用极光 REST v3 按 alias 下发通知，alias 约定为 userID。
type Client struct {
	cfg  *config.Push
	http *http.Client
}

func NewClient(cfg *config.Push) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 5 * time.Second}}
}

// Push 给一批用户下发同一条通知。
func (c *Client) Push(ctx context.Context, userIDs []string, title, content string, opts *options.Opts) error {
	if len(userIDs) == 0 {
		return nil
	}
	extras := map[string]string{}
	if opts != nil {
		if opts.Ex != "" {
			extras["ex"] = opts.Ex
		}
		if opts.Signal != nil && opts.Signal.ClientMsgID != "" {
			extras["clientMsgID"] = opts.Signal.ClientMsgID
		}
	}
	notification := map[string]any{
		"alert": content,
		"ios": map[string]any{
			"alert": content, "sound": "default", "badge": "+1", "extras": extras,
		},
		"android": map[string]any{
			"alert": content, "title": title, "extras": extras,
		},
	}
	body := map[string]any{
		"platform":     "all",
		"audience":     map[string]any{"alias": userIDs},
		"notification": notification,
		"options":      map[string]any{"apns_production": c.cfg.IOSPush.Production},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode jpush request: %w", err)
	}
	pushURL := c.cfg.JPush.PushURL
	if pushURL == "" {
		pushURL = defaultPushURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pushURL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("jpush request: %w", err)
	}
	credential := base64.StdEncoding.EncodeToString([]byte(c.cfg.JPush.AppKey + ":" + c.cfg.JPush.MasterSecret))
	req.Header.Set("Authorization", "Basic "+credential)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jpush post: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read jpush response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jpush status %d: %s", resp.StatusCode, string(data))
	}
	var result struct {
		SendNo string `json:"sendno"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode jpush response: %w", err)
	}
	if result.Error != nil {
		return fmt.Errorf("jpush code %d: %s", result.Error.Code, result.Error.Message)
	}
	return nil
}

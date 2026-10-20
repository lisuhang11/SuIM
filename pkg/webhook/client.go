package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

const defaultTimeoutSec = 5

// Client posts callback commands to one webhook base URL.
type Client struct {
	http     *http.Client
	url      string
	queue    *MemoryQueue
	ownQueue bool
}

func NewWebhookClient(rawURL string, options ...*MemoryQueue) *Client {
	var queue *MemoryQueue
	own := false
	if len(options) > 0 && options[0] != nil {
		queue = options[0]
	} else {
		queue = NewMemoryQueue(webhookWorkerCount, webhookBufferSize)
		own = true
	}
	return &Client{
		http:     newHTTPClient(),
		url:      strings.TrimRight(rawURL, "/"),
		queue:    queue,
		ownQueue: own,
	}
}

// Close stops the queue created by NewWebhookClient. A queue passed in by the caller stays open.
func (c *Client) Close() {
	if c != nil && c.ownQueue {
		c.queue.Stop()
	}
}

// SyncPost posts command and waits for the callback response.
func (c *Client) SyncPost(ctx context.Context, command string, req CallbackReq, resp CallbackResp, before *BeforeConfig) error {
	if before == nil {
		return errorcode.ErrWebhookData.WithMsg("nil before config")
	}
	return c.post(ctx, command, req, resp, before.Timeout)
}

// AsyncPost enqueues command when the after-webhook is enabled.
func (c *Client) AsyncPost(ctx context.Context, command string, req CallbackReq, resp CallbackResp, after *AfterConfig) {
	if after == nil || !after.Enable {
		return
	}
	c.enqueue(ctx, command, req, resp, after.Timeout, nil)
}

// AsyncPostWithQuery enqueues command and appends queryParams to the callback URL.
func (c *Client) AsyncPostWithQuery(ctx context.Context, command string, req CallbackReq, resp CallbackResp, after *AfterConfig, queryParams map[string]string) {
	if after == nil || !after.Enable {
		return
	}
	c.enqueue(ctx, command, req, resp, after.Timeout, queryParams)
}

func (c *Client) enqueue(ctx context.Context, command string, req CallbackReq, resp CallbackResp, timeout int, queryParams map[string]string) {
	err := c.queue.Push(func() {
		var postErr error
		if len(queryParams) == 0 {
			postErr = c.post(ctx, command, req, resp, timeout)
		} else {
			postErr = c.postWithQuery(ctx, command, req, resp, timeout, queryParams)
		}
		if postErr != nil {
			slog.Error("webhook async post", "command", command, "err", postErr)
		}
	})
	if err != nil {
		slog.Error("webhook enqueue", "command", command, "err", err)
	}
}

func (c *Client) post(ctx context.Context, command string, input any, output CallbackResp, timeout int) error {
	fullURL := c.commandURL(command)
	slog.Info("webhook", "url", fullURL, "timeout", timeout)
	if err := c.doPost(ctx, fullURL, input, output, timeout); err != nil {
		return err
	}
	slog.Info("webhook success", "url", fullURL)
	return nil
}

func (c *Client) postWithQuery(ctx context.Context, command string, input any, output CallbackResp, timeout int, queryParams map[string]string) error {
	fullURL := c.commandURL(command)
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return errorcode.ErrWebhookNetwork.WithMsg(err.Error())
	}
	query := parsed.Query()
	for key, value := range queryParams {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	fullURL = parsed.String()
	slog.Info("webhook", "url", fullURL, "timeout", timeout)
	if err := c.doPost(ctx, fullURL, input, output, timeout); err != nil {
		return err
	}
	slog.Info("webhook success", "url", fullURL)
	return nil
}

func (c *Client) commandURL(command string) string {
	return c.url + "/" + command
}

func (c *Client) doPost(ctx context.Context, fullURL string, input any, output CallbackResp, timeoutSec int) error {
	if output == nil {
		return errorcode.ErrWebhookData.WithMsg("nil callback response")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return errorcode.ErrWebhookData.WithMsg("encode callback request")
	}
	if timeoutSec <= 0 {
		timeoutSec = defaultTimeoutSec
	}
	// The caller context may already be cancelled when an async post runs. Keep its values and bound the HTTP call separately.
	reqCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return errorcode.ErrWebhookNetwork.WithMsg(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	if id := operationID(ctx); id != "" {
		req.Header.Set(HeaderOperationID, id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return errorcode.ErrWebhookNetwork.WithMsg(err.Error())
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errorcode.ErrWebhookNetwork.WithMsg(err.Error())
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return errorcode.ErrWebhookNetwork.WithMsg(fmt.Sprintf("status %d", resp.StatusCode))
	}
	if err := json.Unmarshal(b, output); err != nil {
		return errorcode.ErrWebhookData.WithMsg("response format error")
	}
	return output.Parse()
}

func newHTTPClient() *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	var transport *http.Transport
	if ok {
		transport = base.Clone()
	} else {
		transport = &http.Transport{}
	}
	transport.MaxConnsPerHost = 100
	return &http.Client{Transport: transport}
}

package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// LogSender records that a code was handed to the log provider.
// The code is included only when DebugCode is set.
type LogSender struct {
	DebugCode bool
}

func (s LogSender) Send(_ context.Context, phone, code, scene string) (string, error) {
	masked := maskE164(phone)
	slog.Info("sms sent", "provider", "log", "phone", masked, "scene", scene)
	if s.DebugCode {
		slog.Info("sms debug code", "phone", masked, "scene", scene, "code", code)
	}
	return "log", nil
}

// HTTPSender POSTs the code to a provider URL and stores nothing itself.
type HTTPSender struct {
	URL    string
	Client *http.Client
}

func (s HTTPSender) Send(ctx context.Context, phone, code, scene string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"phone": phone,
		"code":  code,
		"scene": scene,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("sms provider status %d", resp.StatusCode)
	}
	return "", nil
}

// HTTPCaptcha asks the configured verifier whether ticket belongs to this send.
type HTTPCaptcha struct {
	URL    string
	Client *http.Client
}

func (c HTTPCaptcha) Verify(ctx context.Context, ticket, phone, scene, ip string) error {
	if strings.TrimSpace(ticket) == "" {
		return fmt.Errorf("captcha ticket is empty")
	}
	body, err := json.Marshal(map[string]string{
		"ticket": ticket,
		"phone":  phone,
		"scene":  scene,
		"ip":     ip,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("captcha status %d", resp.StatusCode)
	}
	return nil
}

func (s HTTPSender) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 5 * time.Second}
}

func (c HTTPCaptcha) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 5 * time.Second}
}

func maskE164(phone string) string {
	digits := strings.TrimPrefix(phone, "+")
	if len(digits) < 8 {
		return "****"
	}
	return "+" + digits[:len(digits)-8] + "****" + digits[len(digits)-4:]
}

// Package livekit signs room tokens for a locally run LiveKit server.
// SuIM stores the server address and API secret. It does not carry audio.
package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const tokenTTL = 10 * time.Minute

// Config is the LiveKit address and the key pair used to sign join tokens.
type Config struct {
	URL    string `yaml:"url"`
	Key    string `yaml:"key"`
	Secret string `yaml:"secret"`
}

func (c Config) Validate() error {
	if c.URL == "" && c.Key == "" && c.Secret == "" {
		return nil
	}
	if c.URL == "" || c.Key == "" || c.Secret == "" {
		return fmt.Errorf("livekit url, key, and secret are required together")
	}
	if len(c.Secret) < 32 {
		return fmt.Errorf("livekit secret is too short")
	}
	return nil
}

// Enabled reports whether token signing is configured.
func (c Config) Enabled() bool {
	return c.URL != "" && c.Key != "" && c.Secret != ""
}

// Sign returns a microphone-only join token for one user and room.
// The URL is the LiveKit signal address the client should open.
func (c Config) Sign(identity, room string, now time.Time) (url, token string, err error) {
	return c.sign(identity, room, now, []string{"microphone"})
}

// SignCall returns a join token for a private audio or video call.
// Video may publish a microphone and a camera. Audio stays microphone-only.
func (c Config) SignCall(identity, room, media string, now time.Time) (url, token string, err error) {
	switch media {
	case "audio":
		return c.sign(identity, room, now, []string{"microphone"})
	case "video":
		return c.sign(identity, room, now, []string{"microphone", "camera"})
	default:
		return "", "", fmt.Errorf("unsupported call media")
	}
}

func (c Config) sign(identity, room string, now time.Time, sources []string) (url, token string, err error) {
	if !c.Enabled() {
		return "", "", fmt.Errorf("livekit is not configured")
	}
	if identity == "" || room == "" {
		return "", "", fmt.Errorf("identity and room are required")
	}
	if len(sources) == 0 {
		return "", "", fmt.Errorf("publish sources are required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	yes := true
	body, err := json.Marshal(claims{
		Issuer:    c.Key,
		Subject:   identity,
		Expires:   now.Add(tokenTTL).Unix(),
		NotBefore: now.Add(-10 * time.Second).Unix(),
		Video: videoGrant{
			RoomJoin:          true,
			Room:              room,
			CanPublish:        &yes,
			CanSubscribe:      &yes,
			CanPublishSources: sources,
		},
	})
	if err != nil {
		return "", "", err
	}
	unsigned := encode([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + encode(body)
	mac := hmac.New(sha256.New, []byte(c.Secret))
	_, _ = mac.Write([]byte(unsigned))
	return c.URL, unsigned + "." + encode(mac.Sum(nil)), nil
}

type claims struct {
	Issuer    string     `json:"iss"`
	Subject   string     `json:"sub"`
	Expires   int64      `json:"exp"`
	NotBefore int64      `json:"nbf"`
	Video     videoGrant `json:"video"`
}

type videoGrant struct {
	RoomJoin          bool     `json:"roomJoin,omitempty"`
	Room              string   `json:"room,omitempty"`
	CanPublish        *bool    `json:"canPublish,omitempty"`
	CanSubscribe      *bool    `json:"canSubscribe,omitempty"`
	CanPublishSources []string `json:"canPublishSources,omitempty"`
}

func encode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// Parse checks the HMAC and returns the identity and room. Tests and local checks use it.
func Parse(secret, token string) (identity, room string, err error) {
	got, err := parseClaims(secret, token)
	if err != nil {
		return "", "", err
	}
	return got.Subject, got.Video.Room, nil
}

// Sources returns the tracks a token is allowed to publish.
func Sources(secret, token string) ([]string, error) {
	got, err := parseClaims(secret, token)
	if err != nil {
		return nil, err
	}
	return got.Video.CanPublishSources, nil
}

func parseClaims(secret, token string) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, fmt.Errorf("invalid livekit token")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return claims{}, fmt.Errorf("invalid livekit token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims{}, fmt.Errorf("invalid livekit token")
	}
	var got claims
	if err := json.Unmarshal(payload, &got); err != nil {
		return claims{}, fmt.Errorf("invalid livekit token")
	}
	return got, nil
}

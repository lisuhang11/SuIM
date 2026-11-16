package config

import (
	"os"
	"strconv"
	"strings"
)

const envPrefix = "SUIMENV_"

// envPaths lists the values that may be injected from the environment. Only
// connection details and secrets are overridable; the whole file stays the
// source of truth for business settings.
var envPaths = map[string][]string{
	"SUIMENV_MYSQL_ADDRESS":           {"mysql", "address"},
	"SUIMENV_MYSQL_USERNAME":          {"mysql", "username"},
	"SUIMENV_MYSQL_PASSWORD":          {"mysql", "password"},
	"SUIMENV_MYSQL_DATABASE":          {"mysql", "database"},
	"SUIMENV_REDIS_ADDRESS":           {"redis", "address"},
	"SUIMENV_REDIS_PASSWORD":          {"redis", "password"},
	"SUIMENV_JWT_SECRET":              {"jwt", "secret"},
	"SUIMENV_SMS_PEPPER":              {"sms", "pepper"},
	"SUIMENV_MINIO_ENDPOINT":          {"minio", "endpoint"},
	"SUIMENV_MINIO_SIGNENDPOINT":      {"minio", "signEndpoint"},
	"SUIMENV_MINIO_ACCESSKEY":         {"minio", "accessKey"},
	"SUIMENV_MINIO_SECRETKEY":         {"minio", "secretKey"},
	"SUIMENV_LIVEKIT_URL":             {"livekit", "url"},
	"SUIMENV_LIVEKIT_KEY":             {"livekit", "key"},
	"SUIMENV_LIVEKIT_SECRET":          {"livekit", "secret"},
	"SUIMENV_WEBHOOKS_URL":            {"webhooks", "url"},
	"SUIMENV_PUSH_GETUI_APPKEY":       {"push", "getui", "appKey"},
	"SUIMENV_PUSH_GETUI_MASTERSECRET": {"push", "getui", "masterSecret"},
	"SUIMENV_PUSH_JPUSH_APPKEY":       {"push", "jpush", "appKey"},
	"SUIMENV_PUSH_JPUSH_MASTERSECRET": {"push", "jpush", "masterSecret"},
}

// applyEnvOverrides replaces yaml values with SUIMENV_* variables. A variable
// is ignored when its target section is absent from the file.
func applyEnvOverrides(raw map[string]any) {
	for name, path := range envPaths {
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			continue
		}
		parent := lookupParent(raw, path[:len(path)-1])
		if parent == nil {
			continue
		}
		key := path[len(path)-1]
		parent[key] = coerce(parent[key], value)
	}
}

func lookupParent(raw map[string]any, parts []string) map[string]any {
	current := raw
	for _, part := range parts {
		next, ok := current[part]
		if !ok {
			return nil
		}
		child, ok := next.(map[string]any)
		if !ok {
			return nil
		}
		current = child
	}
	return current
}

// coerce keeps the yaml value type so the second unmarshal still succeeds.
// A list is written as a comma separated string.
func coerce(current any, value string) any {
	switch current.(type) {
	case bool:
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	case int:
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	case int64:
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	case float64:
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	case []any:
		out := make([]any, 0)
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return value
}

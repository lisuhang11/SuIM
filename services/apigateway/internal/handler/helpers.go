// Package handler 提供 REST handler 通用工具函数。
package handler

import (
	"strconv"
	"strings"
)

// splitComma 分割逗号分隔的字符串。
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// parseInt32 解析 int32，失败返回 fallback。
func parseInt32(s string, fallback int32) int32 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(v)
}

// parseInt64 解析 int64，失败返回 fallback。
func parseInt64(s string, fallback int64) int64 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

// parseInt32Slice 解析逗号分隔的 int32 列表，如 "0,1,-1"。
func parseInt32Slice(s string) []int32 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]int32, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 32)
		if err == nil {
			result = append(result, int32(v))
		}
	}
	return result
}

// parseInt64Slice 解析逗号分隔的 int64 列表。
func parseInt64Slice(s string) []int64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]int64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err == nil {
			result = append(result, v)
		}
	}
	return result
}

// parseBool 解析布尔 query 参数，仅 "true"/"1" 返回 true。
func parseBool(s string) bool {
	return s == "true" || s == "1"
}

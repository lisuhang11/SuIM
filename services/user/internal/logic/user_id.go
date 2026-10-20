package logic

import (
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func normalizeUserID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 || strings.Contains(id, ":") {
		return "", errorcode.ErrUserInvalidUserID
	}
	return id, nil
}

package logic

import (
	"strings"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func normalizeUserID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 || strings.Contains(id, ":") {
		return "", errorcode.ErrRelationInvalidUserID
	}
	return id, nil
}

func normalizeMessage(msg string) (string, error) {
	msg = strings.TrimSpace(msg)
	if utf8.RuneCountInString(msg) > 512 {
		return "", errorcode.ErrRelationInvalidMessage
	}
	return msg, nil
}

func normalizeRemark(remark string) (string, error) {
	remark = strings.TrimSpace(remark)
	if utf8.RuneCountInString(remark) > 255 {
		return "", errorcode.ErrRelationInvalidRemark
	}
	return remark, nil
}

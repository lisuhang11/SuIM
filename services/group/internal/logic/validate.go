package logic

import (
	"strings"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

func normalizeUserID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 || strings.Contains(id, ":") {
		return "", errorcode.ErrGroupInvalidUserID
	}
	return id, nil
}

func normalizeUserIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, errorcode.ErrGroupInvalidArgs.WithMsg("user ids are empty")
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		id, err := normalizeUserID(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			return nil, errorcode.ErrGroupMemberRepeated
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func normalizeGroupID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 {
		return "", errorcode.ErrGroupInvalidArgs.WithMsg("group_id is invalid")
	}
	return id, nil
}

func normalizeName(name string, required bool) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		if required {
			return "", errorcode.ErrGroupInvalidName
		}
		return "", nil
	}
	if utf8.RuneCountInString(name) > 255 {
		return "", errorcode.ErrGroupInvalidName
	}
	return name, nil
}

func normalizeLimited(value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > max {
		return "", errorcode.ErrGroupInvalidArgs.WithMsg("field is too long")
	}
	return value, nil
}

func normalizeGroupType(v int32) (int32, error) {
	if v == 0 {
		return model.GroupTypeNormal, nil
	}
	switch v {
	case model.GroupTypeNormal, model.GroupTypeSuper:
		return v, nil
	default:
		return 0, errorcode.ErrGroupTypeUnsupported
	}
}

func normalizeFlag(v int32) error {
	if v != 0 && v != 1 {
		return errorcode.ErrGroupInvalidArgs.WithMsg("flag must be 0 or 1")
	}
	return nil
}

func normalizeVerification(v int32) error {
	switch v {
	case model.NeedVerifyInviteDirect, model.NeedVerifyAll, 2:
		return nil
	default:
		return errorcode.ErrGroupInvalidArgs.WithMsg("need_verification is invalid")
	}
}

func page(offset, limit int32) (int, int, error) {
	if offset < 0 || limit < 0 || limit > 200 {
		return 0, 0, errorcode.ErrGroupInvalidArgs.WithMsg("pagination is invalid")
	}
	if limit == 0 {
		limit = 20
	}
	return int(offset), int(limit), nil
}

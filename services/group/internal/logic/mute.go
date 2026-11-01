package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

func muteOperator(groupID, opUserID string) (string, string, error) {
	groupID, err := normalizeGroupID(groupID)
	if err != nil {
		return "", "", err
	}
	op, err := normalizeUserID(opUserID)
	if err != nil {
		return "", "", err
	}
	return groupID, op, nil
}

func muteUsers(groupID, opUserID, userID string) (string, string, string, error) {
	groupID, op, err := muteOperator(groupID, opUserID)
	if err != nil {
		return "", "", "", err
	}
	userID, err = normalizeUserID(userID)
	if err != nil {
		return "", "", "", err
	}
	return groupID, op, userID, nil
}

func (s *Server) authorizeMute(ctx context.Context, groupID, op, userID string) error {
	opMember, err := s.memberOf(ctx, groupID, op)
	if err != nil {
		if errors.Is(err, errorcode.ErrGroupNotMember) {
			return errorcode.ErrGroupNoPermission
		}
		return err
	}
	target, err := s.memberOf(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !canMuteMember(opMember, target) {
		return errorcode.ErrGroupNoPermission
	}
	return nil
}

func canMuteMember(op, target *model.Member) bool {
	switch target.RoleLevel {
	case model.RoleOwner:
		return false
	case model.RoleAdmin:
		return op.RoleLevel == model.RoleOwner
	default:
		return op.RoleLevel == model.RoleOwner || op.RoleLevel == model.RoleAdmin
	}
}

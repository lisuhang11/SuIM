package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/grouppb"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// QuitGroup 成员退出群。群主不能退群。
func (s *Server) QuitGroup(ctx context.Context, req *grouppb.QuitGroupReq) (*grouppb.QuitGroupResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if member.RoleLevel == model.RoleOwner {
		return nil, errorcode.ErrGroupOwnerQuit
	}
	if err := s.store.DeleteMember(ctx, groupID, userID, s.now()); err != nil {
		return nil, mapStore(err)
	}
	// 退群通知、会话 seq 和 webhook 尚未接入。
	return &grouppb.QuitGroupResp{}, nil
}

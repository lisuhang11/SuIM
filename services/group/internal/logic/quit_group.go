package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// QuitGroup 成员退出群。群主不能退群。
func (s *Server) QuitGroup(ctx context.Context, req *grouppb.QuitGroupReq) (*grouppb.QuitGroupResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	userID, err := bindActor(ctx, req.GetUserId())
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
	group, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		return nil, mapStore(err)
	}
	if err := s.store.DeleteMember(ctx, groupID, userID, s.now()); err != nil {
		return nil, mapStore(err)
	}
	s.notifyQuit(ctx, group, member)
	if err := s.freezeHistory(ctx, group, []string{userID}); err != nil {
		return nil, err
	}
	s.afterQuit(ctx, groupID, userID)
	return &grouppb.QuitGroupResp{}, nil
}

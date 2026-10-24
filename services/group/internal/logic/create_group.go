package logic

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// CreateGroup 创建群，创建者即群主。未传群类型时为普通群。
func (s *Server) CreateGroup(ctx context.Context, req *grouppb.CreateGroupReq) (*grouppb.CreateGroupResp, error) {
	// 1. 1 为普通群，2 为超级群；未传时建普通群。调用方身份尚未从上下文读取，创建者即群主。
	groupType, err := normalizeGroupType(req.GetGroupType())
	if err != nil {
		return nil, err
	}
	owner, err := normalizeUserID(req.GetCreatorUserId())
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(req.GetGroupName(), true)
	if err != nil {
		return nil, err
	}
	notification, err := normalizeLimited(req.GetNotification(), 8000)
	if err != nil {
		return nil, err
	}
	introduction, err := normalizeLimited(req.GetIntroduction(), 1024)
	if err != nil {
		return nil, err
	}
	faceURL, err := normalizeLimited(req.GetFaceUrl(), 512)
	if err != nil {
		return nil, err
	}
	ex, err := normalizeLimited(req.GetEx(), 1024)
	if err != nil {
		return nil, err
	}
	if err := normalizeVerification(req.GetNeedVerification()); err != nil {
		return nil, err
	}
	if err := normalizeFlag(req.GetLookMemberInfo()); err != nil {
		return nil, err
	}
	if err := normalizeFlag(req.GetApplyMemberFriend()); err != nil {
		return nil, err
	}
	memberIDs, err := normalizeOptionalIDs(req.GetMemberIds(), owner)
	if err != nil {
		return nil, err
	}

	// 2. 群主和初始成员都必须是已注册用户。
	profiles, err := s.requireUsers(ctx, append([]string{owner}, memberIDs...))
	if err != nil {
		return nil, err
	}

	// 3. 创建群和成员入群前的 webhook 尚未接入。
	groupID, err := s.newGroupID(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	group := &model.Group{
		GroupID:           groupID,
		GroupName:         name,
		Notification:      notification,
		Introduction:      introduction,
		FaceURL:           faceURL,
		CreateTime:        now,
		Status:            model.GroupStatusOK,
		CreatorUserID:     owner,
		GroupType:         groupType,
		NeedVerification:  req.GetNeedVerification(),
		LookMemberInfo:    req.GetLookMemberInfo(),
		ApplyMemberFriend: req.GetApplyMemberFriend(),
		Ex:                ex,
	}
	if notification != "" {
		group.NotificationUpdateTime = now
		group.NotificationUserID = owner
	}
	members := []*model.Member{newMember(groupID, owner, profiles[owner], model.RoleOwner, owner, now)}
	for _, id := range memberIDs {
		members = append(members, newMember(groupID, id, profiles[id], model.RoleOrdinary, owner, now))
	}
	if err := s.store.CreateGroup(ctx, group, members, now); err != nil {
		return nil, mapStore(err)
	}

	// 4. 公告通知和创建后 webhook 尚未接入。建群通知带上成员版本，由 msggateway 推给在线成员。
	created, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		return nil, mapStore(err)
	}
	s.notifyCreated(ctx, owner, created, members)
	return &grouppb.CreateGroupResp{GroupId: groupID, Group: groupPB(created)}, nil
}

func (s *Server) newGroupID(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return "", err
		}
		id := new(big.Int).SetBytes(buf[:]).String()
		if id == "0" {
			continue
		}
		exists, err := s.store.GroupExists(ctx, id)
		if err != nil {
			return "", err
		}
		if !exists {
			return id, nil
		}
	}
	return "", errorcode.ErrGroupIDGenFailed
}

func normalizeOptionalIDs(ids []string, owner string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out, err := normalizeUserIDs(ids)
	if err != nil {
		return nil, err
	}
	for _, id := range out {
		if id == owner {
			return nil, errorcode.ErrGroupMemberRepeated
		}
	}
	return out, nil
}

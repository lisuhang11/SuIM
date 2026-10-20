package logic

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) UpdateUser(ctx context.Context, req *userpb.UpdateUserReq) (*userpb.UpdateUserResp, error) {
	// 1. 校验用户 ID、昵称和头像。邮箱与密码不在这里修改。
	in := req.GetUser()
	if in == nil {
		return nil, errorcode.ErrUserInvalidUserID
	}
	id, err := normalizeUserID(in.GetUserId())
	if err != nil {
		return nil, err
	}
	nickname := strings.TrimSpace(in.GetNickname())
	if nickname == "" || utf8.RuneCountInString(nickname) > 255 {
		return nil, errorcode.ErrUserInvalidUsername
	}
	avatar := strings.TrimSpace(in.GetAvatarUrl())
	if len(avatar) > 1024 {
		return nil, errorcode.ErrUserInvalidAvatar
	}
	// 2. 读取当前资料。
	old, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	// 3. 覆盖昵称、头像、管理员级别、收消息选项和启用状态。
	updated := *old
	updated.Nickname = nickname
	updated.AvatarURL = avatar
	updated.AppMangerLevel = in.GetAppMangerLevel()
	updated.GlobalRecvMsgOpt = in.GetGlobalRecvMsgOpt()
	updated.IsActive = in.GetIsActive()
	if err := s.store.UpdateProfile(ctx, &updated); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("update user")
	}
	// 4. 账号被停用时作废全部会话。
	//    kafka: 通知 msggateway 断开该用户全部平台的在线连接。尚未接入。
	if old.IsActive && !updated.IsActive {
		revoked, err := s.store.RevokeUser(ctx, id)
		s.invalidate(ctx, revoked)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("revoke session")
		}
	}
	// 5. 昵称或头像变化时同步资料。尚未接入。
	//    relation rpc: NotificationUserInfoUpdate，刷新好友侧资料。
	//    group rpc: NotificationUserInfoUpdate，刷新群成员侧资料。
	//    kafka: 投递用户资料变更，供 msggateway 推送给在线端。
	return &userpb.UpdateUserResp{}, nil
}

package logic

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) UpdateUser(ctx context.Context, req *userpb.UpdateUserReq) (*userpb.UpdateUserResp, error) {
	// 1. 校验用户 ID，并只接收本次提交的字段。
	id, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	patch, err := userPatch(req)
	if err != nil {
		return nil, err
	}
	if patchEmpty(patch) {
		return nil, errorcode.ErrUserNothingToUpdate
	}
	// 2. 读取当前资料。邮箱与密码不在这里修改。
	old, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	// 3. 只更新提交的列。
	if err := s.store.UpdateProfile(ctx, id, patch); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		if errors.Is(err, store.ErrAccountTaken) {
			return nil, errorcode.ErrUserAccountTaken
		}
		return nil, errorcode.ErrInternal.WithMsg("update user")
	}
	// 4. 账号被停用时作废全部会话。
	//    kafka: 通知 msggateway 断开该用户全部平台的在线连接。尚未接入。
	if old.IsActive && patch.IsActive != nil && !*patch.IsActive {
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

func userPatch(req *userpb.UpdateUserReq) (model.UserPatch, error) {
	var patch model.UserPatch
	if req.Nickname != nil {
		nickname := strings.TrimSpace(req.GetNickname())
		if nickname == "" || utf8.RuneCountInString(nickname) > 255 {
			return patch, errorcode.ErrUserInvalidUsername
		}
		patch.Nickname = &nickname
	}
	if req.AvatarUrl != nil {
		avatar := strings.TrimSpace(req.GetAvatarUrl())
		if len(avatar) > 1024 {
			return patch, errorcode.ErrUserInvalidAvatar
		}
		patch.AvatarURL = &avatar
	}
	if req.AppMangerLevel != nil {
		level := req.GetAppMangerLevel()
		patch.AppMangerLevel = &level
	}
	if req.GlobalRecvMsgOpt != nil {
		opt := req.GetGlobalRecvMsgOpt()
		patch.GlobalRecvMsgOpt = &opt
	}
	if req.IsActive != nil {
		active := req.GetIsActive()
		patch.IsActive = &active
	}
	if req.AreaCode != nil {
		code := strings.TrimSpace(req.GetAreaCode())
		if len(code) > 16 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("area_code is too long")
		}
		patch.AreaCode = &code
	}
	if req.PhoneNumber != nil {
		phone := strings.TrimSpace(req.GetPhoneNumber())
		if len(phone) > 32 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("phone_number is too long")
		}
		patch.PhoneNumber = &phone
	}
	if req.Account != nil {
		account := strings.TrimSpace(req.GetAccount())
		if len(account) > 64 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("account is too long")
		}
		patch.Account = &account
	}
	if req.Gender != nil {
		gender := req.GetGender()
		if gender < 0 || gender > 2 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("gender must be 0, 1 or 2")
		}
		patch.Gender = &gender
	}
	if req.BirthTime != nil {
		ms := req.GetBirthTime()
		if ms < 0 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("birth_time is invalid")
		}
		if ms == 0 {
			patch.ClearBirth = true
		} else {
			birth := time.UnixMilli(ms)
			patch.BirthTime = &birth
		}
	}
	if req.Level != nil {
		level := req.GetLevel()
		if level < 0 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("level is invalid")
		}
		patch.Level = &level
	}
	if req.AllowVibration != nil {
		value, err := allowOpt(req.GetAllowVibration())
		if err != nil {
			return patch, err
		}
		patch.AllowVibration = &value
	}
	if req.AllowBeep != nil {
		value, err := allowOpt(req.GetAllowBeep())
		if err != nil {
			return patch, err
		}
		patch.AllowBeep = &value
	}
	if req.AllowAddFriend != nil {
		value, err := allowOpt(req.GetAllowAddFriend())
		if err != nil {
			return patch, err
		}
		patch.AllowAddFriend = &value
	}
	if req.RegisterType != nil {
		kind := req.GetRegisterType()
		if kind < 0 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("register_type is invalid")
		}
		patch.RegisterType = &kind
	}
	if req.Ex != nil {
		ex := req.GetEx()
		if len(ex) > 1024 {
			return patch, errorcode.ErrUserInvalidProfile.WithMsg("ex is too long")
		}
		patch.Ex = &ex
	}
	return patch, nil
}

func allowOpt(value int32) (int32, error) {
	if value != 1 && value != 2 {
		return 0, errorcode.ErrUserInvalidProfile.WithMsg("allow flag must be 1 or 2")
	}
	return value, nil
}

func patchEmpty(patch model.UserPatch) bool {
	return patch.Nickname == nil && patch.AvatarURL == nil && patch.AppMangerLevel == nil &&
		patch.GlobalRecvMsgOpt == nil && patch.IsActive == nil && patch.AreaCode == nil &&
		patch.PhoneNumber == nil && patch.Account == nil && patch.Gender == nil &&
		!patch.ClearBirth && patch.BirthTime == nil && patch.Level == nil &&
		patch.AllowVibration == nil && patch.AllowBeep == nil && patch.AllowAddFriend == nil &&
		patch.RegisterType == nil && patch.Ex == nil
}

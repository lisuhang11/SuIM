package logic

import (
	"context"
	"encoding/json"
	"log/slog"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const (
	msgFromSystem = 200
	sessionSingle = 1

	contentFriendApproved = 1201
	contentFriendRejected = 1202
	contentFriendApply    = 1203
	contentFriendDeleted  = 1205
	contentFriendRemark   = 1206
	contentBlackAdded     = 1207
	contentBlackDeleted   = 1208

	optNotNotification = "isNotNotification"
	optHistory         = "history"
	optUnreadCount     = "unreadCount"
	optSendMsg         = "isSendMsg"
	optOfflinePush     = "offlinePush"
)

// friendNoticeOptions 按 OpenIM 的默认通知配置。
// 只有同意会进单聊并计未读。通知副本不落历史。申请、同意、拒绝打开离线推送。
func friendNoticeOptions(sendMessage, offline bool) map[string]bool {
	return map[string]bool{
		optNotNotification: false,
		optSendMsg:         sendMessage,
		optUnreadCount:     sendMessage,
		optHistory:         false,
		optOfflinePush:     offline,
	}
}

func (s *Server) notifyFriendApply(ctx context.Context, from, to string) {
	s.sendFriendNotice(ctx, from, to, contentFriendApply, friendApplicationTips{
		FromToUserID: &fromToUserID{FromUserID: from, ToUserID: to},
		Request:      s.friendRequestTip(ctx, from, to),
	}, false, true)
}

func (s *Server) notifyFriendAgreed(ctx context.Context, from, to, handleMsg string) {
	s.sendFriendNotice(ctx, to, from, contentFriendApproved, friendApprovedTips{
		FromToUserID: &fromToUserID{FromUserID: from, ToUserID: to},
		HandleMsg:    handleMsg,
		Request:      s.friendRequestTip(ctx, from, to),
	}, true, true)
}

func (s *Server) notifyFriendRefused(ctx context.Context, from, to, handleMsg string) {
	s.sendFriendNotice(ctx, to, from, contentFriendRejected, friendRejectedTips{
		FromToUserID: &fromToUserID{FromUserID: from, ToUserID: to},
		HandleMsg:    handleMsg,
		Request:      s.friendRequestTip(ctx, from, to),
	}, false, true)
}

func (s *Server) notifyFriendDeleted(ctx context.Context, owner, friend string) {
	s.sendFriendNotice(ctx, owner, friend, contentFriendDeleted, friendDeletedTips{
		FromToUserID: &fromToUserID{FromUserID: owner, ToUserID: friend},
	}, false, false)
}

func (s *Server) notifyFriendRemark(ctx context.Context, owner, friend string) {
	tips := friendRemarkTips{FromToUserID: &fromToUserID{FromUserID: owner, ToUserID: friend}}
	if head, err := s.store.FriendVersion(ctx, owner); err != nil {
		slog.Warn("friend version", "err", err, "userID", owner)
	} else {
		tips.FriendVersion = head.Version
		tips.FriendVersionID = head.ID
	}
	if sortVersion, err := s.store.FriendSortVersion(ctx, owner); err != nil {
		slog.Warn("friend sort version", "err", err, "userID", owner)
	} else {
		tips.FriendSortVersion = sortVersion
	}
	s.sendFriendNotice(ctx, owner, friend, contentFriendRemark, tips, false, false)
}

func (s *Server) notifyBlackAdded(ctx context.Context, owner, block string) {
	s.sendFriendNotice(ctx, owner, block, contentBlackAdded, blackTips{
		FromToUserID: &fromToUserID{FromUserID: owner, ToUserID: block},
	}, false, false)
}

func (s *Server) notifyBlackDeleted(ctx context.Context, owner, block string) {
	s.sendFriendNotice(ctx, owner, block, contentBlackDeleted, blackTips{
		FromToUserID: &fromToUserID{FromUserID: owner, ToUserID: block},
	}, false, false)
}

func (s *Server) sendFriendNotice(ctx context.Context, sendID, recvID string, contentType int32, detail any, sendMessage, offline bool) {
	if s.sender == nil || sendID == "" || recvID == "" {
		return
	}
	body, err := json.Marshal(detail)
	if err != nil {
		slog.Warn("friend notice", "err", err, "contentType", contentType)
		return
	}
	content, err := json.Marshal(struct {
		Detail string `json:"detail"`
	}{Detail: string(body)})
	if err != nil {
		slog.Warn("friend notice", "err", err, "contentType", contentType)
		return
	}
	now := s.now().UnixMilli()
	msg := &msgpb.MsgData{
		SendId:      sendID,
		RecvId:      recvID,
		SessionType: sessionSingle,
		MsgFrom:     msgFromSystem,
		ContentType: contentType,
		Content:     content,
		SendTime:    now,
		CreateTime:  now,
		Options:     friendNoticeOptions(sendMessage, offline),
	}
	if offline {
		title := friendOfflineTitle(contentType)
		msg.OfflinePushInfo = &msgpb.OfflinePushInfo{Title: title, Desc: title, Ex: title}
	}
	if err := s.sender.SendNotification(ctx, msg); err != nil {
		slog.Warn("friend notice", "err", err, "sendID", sendID, "recvID", recvID, "contentType", contentType)
	}
}

func friendOfflineTitle(contentType int32) string {
	switch contentType {
	case contentFriendApply:
		return "Somebody applies to add you as a friend"
	case contentFriendApproved:
		return "Someone applies to add your friend application"
	case contentFriendRejected:
		return "Someone rejected your friend application"
	default:
		return ""
	}
}

func (s *Server) friendRequestTip(ctx context.Context, from, to string) *friendRequestTip {
	row, err := s.store.TakeFriendRequest(ctx, from, to)
	if err != nil {
		slog.Warn("friend request notice", "err", err, "from", from, "to", to)
		return nil
	}
	tip := &friendRequestTip{
		FromUserID:    row.FromUserID,
		ToUserID:      row.ToUserID,
		HandleResult:  row.HandleResult,
		ReqMsg:        row.ReqMsg,
		CreateTime:    millis(row.CreateTime),
		HandlerUserID: row.HandlerUserID,
		HandleMsg:     row.HandleMsg,
		HandleTime:    millis(row.HandleTime),
	}
	if s.users == nil {
		return tip
	}
	profiles, err := s.users.GetUsers(ctx, []string{from, to})
	if err != nil {
		slog.Warn("friend request profiles", "err", err)
		return tip
	}
	if profile, ok := profiles[from]; ok {
		tip.FromNickname = profile.Nickname
		tip.FromFaceURL = profile.AvatarURL
	}
	if profile, ok := profiles[to]; ok {
		tip.ToNickname = profile.Nickname
		tip.ToFaceURL = profile.AvatarURL
	}
	return tip
}

type fromToUserID struct {
	FromUserID string `json:"fromUserID"`
	ToUserID   string `json:"toUserID"`
}

type friendApplicationTips struct {
	FromToUserID *fromToUserID     `json:"fromToUserID"`
	Request      *friendRequestTip `json:"request,omitempty"`
}

type friendApprovedTips struct {
	FromToUserID *fromToUserID     `json:"fromToUserID"`
	HandleMsg    string            `json:"handleMsg,omitempty"`
	Request      *friendRequestTip `json:"request,omitempty"`
}

type friendRejectedTips struct {
	FromToUserID *fromToUserID     `json:"fromToUserID"`
	HandleMsg    string            `json:"handleMsg,omitempty"`
	Request      *friendRequestTip `json:"request,omitempty"`
}

type friendDeletedTips struct {
	FromToUserID *fromToUserID `json:"fromToUserID"`
}

type friendRemarkTips struct {
	FromToUserID      *fromToUserID `json:"fromToUserID"`
	FriendVersion     uint64        `json:"friendVersion,omitempty"`
	FriendVersionID   string        `json:"friendVersionID,omitempty"`
	FriendSortVersion uint64        `json:"friendSortVersion,omitempty"`
}

type blackTips struct {
	FromToUserID *fromToUserID `json:"fromToUserID"`
}

type friendRequestTip struct {
	FromUserID    string `json:"fromUserID"`
	FromNickname  string `json:"fromNickname,omitempty"`
	FromFaceURL   string `json:"fromFaceURL,omitempty"`
	ToUserID      string `json:"toUserID"`
	ToNickname    string `json:"toNickname,omitempty"`
	ToFaceURL     string `json:"toFaceURL,omitempty"`
	HandleResult  int32  `json:"handleResult"`
	ReqMsg        string `json:"reqMsg,omitempty"`
	CreateTime    int64  `json:"createTime,omitempty"`
	HandlerUserID string `json:"handlerUserID,omitempty"`
	HandleMsg     string `json:"handleMsg,omitempty"`
	HandleTime    int64  `json:"handleTime,omitempty"`
}

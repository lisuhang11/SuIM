// Package handler — BFF 聚合接口（对齐 OpenIM jssdk）。
package handler

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	pbConv "SuIM/proto/conversationpb"
	pbGroup "SuIM/proto/grouppb"
	pbMsg "SuIM/proto/messagepb"
	pbRel "SuIM/proto/relationpb"
	pbUser "SuIM/proto/userpb"

	"github.com/gin-gonic/gin"
)

const (
	defaultActiveConversationCount = 100
	maxActiveConversationCount     = 500
)

// BFFHandler 跨服务编排，面向前端会话列表等聚合读路径。
type BFFHandler struct {
	conversation pbConv.ConversationClient
	message      pbMsg.MessageClient
	user         pbUser.UserServiceClient
	relation     pbRel.RelationServiceClient
	group        pbGroup.GroupServiceClient
}

// NewBFFHandler 创建 BFF handler。
func NewBFFHandler(
	conversation pbConv.ConversationClient,
	message pbMsg.MessageClient,
	user pbUser.UserServiceClient,
	relation pbRel.RelationServiceClient,
	group pbGroup.GroupServiceClient,
) *BFFHandler {
	return &BFFHandler{
		conversation: conversation,
		message:      message,
		user:         user,
		relation:     relation,
		group:        group,
	}
}

// RegisterRoutes 注册 /api/v1/bff 路由。
func (h *BFFHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/active-conversations", h.GetActiveConversations)
}

type activeConversationsReq struct {
	Count int64 `json:"count"`
}

// GetActiveConversations POST /bff/active-conversations
// 编排 conversation + message + user/relation/group，返回兼容前端的 Conversation 列表。
func (h *BFFHandler) GetActiveConversations(c *gin.Context) {
	var body activeConversationsReq
	_ = c.ShouldBindJSON(&body)
	count := body.Count
	if count <= 0 {
		count = defaultActiveConversationCount
	}
	if count > maxActiveConversationCount {
		count = maxActiveConversationCount
	}

	ownerID := userIDFromCtx(c)
	if ownerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "not authenticated"})
		return
	}
	if h.conversation == nil || h.message == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "bff dependencies unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 8*time.Second)
	defer cancel()
	start := time.Now()

	idsResp, err := h.conversation.GetConversationIDs(ctx, &pbConv.GetConversationIDsReq{UserId: ownerID})
	recordGRPC("conversation", "GetConversationIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	allIDs := idsResp.GetConversationIds()
	if len(allIDs) == 0 {
		Respond(c, gin.H{"conversations": []any{}, "unread_total": 0})
		return
	}

	actStart := time.Now()
	activeResp, err := h.message.GetActiveConversation(ctx, &pbMsg.GetActiveConversationReq{
		ConversationIds: allIDs,
	})
	recordGRPC("message", "GetActiveConversation", err, actStart)
	if err != nil {
		RespondError(c, err)
		return
	}
	active := activeResp.GetConversations()
	if len(active) == 0 {
		Respond(c, gin.H{"conversations": []any{}, "unread_total": 0})
		return
	}

	seqStart := time.Now()
	seqResp, seqErr := h.message.GetConversationsHasReadAndMaxSeq(ctx, &pbMsg.GetConversationsHasReadAndMaxSeqReq{
		ConversationIds: allIDs,
	})
	recordGRPC("message", "GetConversationsHasReadAndMaxSeq", seqErr, seqStart)
	seqMap := map[string]*pbMsg.Seqs{}
	if seqErr == nil && seqResp != nil {
		seqMap = seqResp.GetSeqs()
	}

	pinnedSet := map[string]struct{}{}
	pinStart := time.Now()
	if pinResp, pinErr := h.conversation.GetPinnedConversationIDs(ctx, &pbConv.GetPinnedConversationIDsReq{UserId: ownerID}); pinErr == nil {
		for _, id := range pinResp.GetConversationIds() {
			pinnedSet[id] = struct{}{}
		}
	}
	recordGRPC("conversation", "GetPinnedConversationIDs", nil, pinStart)

	sort.SliceStable(active, func(i, j int) bool {
		_, ip := pinnedSet[active[i].GetConversationId()]
		_, jp := pinnedSet[active[j].GetConversationId()]
		if ip != jp {
			return ip
		}
		return active[i].GetLastTime() > active[j].GetLastTime()
	})
	if int64(len(active)) > count {
		active = active[:count]
	}

	topIDs := make([]string, 0, len(active))
	for _, a := range active {
		topIDs = append(topIDs, a.GetConversationId())
	}

	convStart := time.Now()
	convsResp, err := h.conversation.GetConversations(ctx, &pbConv.GetConversationsReq{
		OwnerUserId:     ownerID,
		ConversationIds: topIDs,
	})
	recordGRPC("conversation", "GetConversations", err, convStart)
	if err != nil {
		RespondError(c, err)
		return
	}
	convMap := make(map[string]*pbConv.Conversation, len(convsResp.GetConversations()))
	for _, cv := range convsResp.GetConversations() {
		convMap[cv.GetConversationId()] = cv
	}

	// 按 maxSeq 拉消息；无效/缺失再 GetLastMessage 回退。
	lastBySeq := map[string]*pbMsg.MsgData{}
	needFallback := make([]string, 0)
	for _, a := range active {
		cid := a.GetConversationId()
		maxSeq := a.GetMaxSeq()
		if s, ok := seqMap[cid]; ok && s.GetMaxSeq() > 0 {
			maxSeq = s.GetMaxSeq()
		}
		if maxSeq <= 0 {
			needFallback = append(needFallback, cid)
			continue
		}
		ms, mErr := h.message.GetMessagesBySeq(ctx, &pbMsg.GetMessagesBySeqReq{
			ConversationId: cid,
			Seqs:           []int64{maxSeq},
		})
		if mErr != nil || len(ms.GetMsgData()) == 0 {
			needFallback = append(needFallback, cid)
			continue
		}
		msg := ms.GetMsgData()[0]
		// 用户已删场景由 GetMessagesBySeq 过滤；此处仅做兜底。
		if msg == nil {
			needFallback = append(needFallback, cid)
			continue
		}
		lastBySeq[cid] = msg
	}
	if len(needFallback) > 0 {
		fbStart := time.Now()
		fb, fbErr := h.message.GetLastMessage(ctx, &pbMsg.GetLastMessageReq{
			UserId:          ownerID,
			ConversationIds: needFallback,
		})
		recordGRPC("message", "GetLastMessage", fbErr, fbStart)
		if fbErr == nil {
			for id, m := range fb.GetMsgs() {
				lastBySeq[id] = m
			}
		}
	}

	// 全局未读按全部会话算（与 OpenIM 一致），不仅 Top N。
	var unreadTotal int64
	for _, s := range seqMap {
		if n := s.GetMaxSeq() - s.GetHasReadSeq(); n > 0 {
			unreadTotal += n
		}
	}

	peerIDs := make([]string, 0)
	groupIDs := make([]string, 0)
	for _, id := range topIDs {
		cv := convMap[id]
		if cv == nil {
			continue
		}
		if cv.GetConversationType() == 2 {
			gid := cv.GetGroupId()
			if gid == "" {
				gid = strings.TrimPrefix(id, "gid_")
			}
			if gid != "" {
				groupIDs = append(groupIDs, gid)
			}
		} else if uid := cv.GetUserId(); uid != "" {
			peerIDs = append(peerIDs, uid)
		}
	}

	userMap := map[string]*pbUser.UserInfo{}
	friendMap := map[string]*pbRel.FriendInfo{}
	groupMap := map[string]*pbGroup.GroupInfo{}

	if len(peerIDs) > 0 && h.user != nil {
		uStart := time.Now()
		uResp, uErr := h.user.GetUsersByIDs(ctx, &pbUser.GetUsersByIDsReq{UserIds: peerIDs})
		recordGRPC("user", "GetUsersByIDs", uErr, uStart)
		if uErr == nil {
			for id, u := range uResp.GetUsers() {
				userMap[id] = u
			}
		}
	}
	if len(peerIDs) > 0 && h.relation != nil {
		rStart := time.Now()
		// 拉取好友列表以取备注；Top N 场景体量可接受。
		fResp, fErr := h.relation.GetFriends(ctx, &pbRel.GetFriendsReq{UserId: ownerID, Offset: 0, Limit: 10000})
		recordGRPC("relation", "GetFriends", fErr, rStart)
		if fErr == nil {
			want := map[string]struct{}{}
			for _, id := range peerIDs {
				want[id] = struct{}{}
			}
			for _, f := range fResp.GetFriends() {
				if _, ok := want[f.GetFriendUserId()]; ok {
					friendMap[f.GetFriendUserId()] = f
				}
			}
		}
	}
	if len(groupIDs) > 0 && h.group != nil {
		gStart := time.Now()
		gResp, gErr := h.group.GetGroupsInfo(ctx, &pbGroup.GetGroupsInfoReq{GroupIds: groupIDs})
		recordGRPC("group", "GetGroupsInfo", gErr, gStart)
		if gErr == nil {
			for _, g := range gResp.GetGroups() {
				groupMap[g.GetGroupId()] = g
			}
		}
	}

	out := make([]gin.H, 0, len(topIDs))
	for _, a := range active {
		cid := a.GetConversationId()
		cv := convMap[cid]
		if cv == nil {
			continue
		}
		maxSeq := a.GetMaxSeq()
		var readSeq int64
		if s, ok := seqMap[cid]; ok {
			if s.GetMaxSeq() > 0 {
				maxSeq = s.GetMaxSeq()
			}
			readSeq = s.GetHasReadSeq()
		}
		unread := maxSeq - readSeq
		if unread < 0 {
			unread = 0
		}

		item := gin.H{
			"conversation_id":   cid,
			"conversation_type": cv.GetConversationType(),
			"user_id":           cv.GetUserId(),
			"group_id":          cv.GetGroupId(),
			"is_pinned":         cv.GetIsPinned(),
			"recv_msg_opt":      cv.GetRecvMsgOpt(),
			"is_muted":          cv.GetRecvMsgOpt() == 1,
			"unread_count":      unread,
			"create_time":       cv.GetCreateTime(),
			"title":             "",
			"avatar":            "",
		}
		if cv.GetConversationType() == 2 {
			gid := cv.GetGroupId()
			if gid == "" {
				gid = strings.TrimPrefix(cid, "gid_")
			}
			item["group_id"] = gid
			if g := groupMap[gid]; g != nil {
				item["title"] = g.GetGroupName()
				item["avatar"] = g.GetFaceUrl()
			}
		} else {
			peer := cv.GetUserId()
			title, avatar := peer, ""
			if f := friendMap[peer]; f != nil {
				if r := strings.TrimSpace(f.GetRemark()); r != "" {
					title = r
				} else if n := strings.TrimSpace(f.GetNickname()); n != "" {
					title = n
				}
				if f.GetAvatarUrl() != "" {
					avatar = f.GetAvatarUrl()
				}
			}
			if u := userMap[peer]; u != nil {
				if title == peer || title == "" {
					if n := strings.TrimSpace(u.GetNickname()); n != "" {
						title = n
					}
				}
				if avatar == "" {
					avatar = u.GetAvatarUrl()
				}
			}
			item["title"] = title
			item["avatar"] = avatar
		}
		if m := lastBySeq[cid]; m != nil {
			item["msg_info"] = gin.H{
				"server_msg_id":        m.GetServerMsgId(),
				"client_msg_id":        m.GetClientMsgId(),
				"session_type":         m.GetSessionType(),
				"send_id":              m.GetSendId(),
				"recv_id":              m.GetRecvId(),
				"sender_name":          m.GetSenderNickname(),
				"face_url":             m.GetSenderFaceUrl(),
				"group_id":             m.GetGroupId(),
				"latest_msg_recv_time": m.GetSendTime(),
				"msg_from":             m.GetMsgFrom(),
				"content_type":         m.GetContentType(),
				"content":              m.GetContent(),
				"ex":                   m.GetEx(),
			}
		}
		out = append(out, item)
	}

	Respond(c, gin.H{
		"conversations": out,
		"unread_total":  unreadTotal,
	})
}

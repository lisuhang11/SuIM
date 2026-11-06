package ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	"github.com/lisuhang11/SuIM/pkg/online"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/proto/group"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
)

// TokenChecker validates a login token and returns the user and platform stored in it.
type TokenChecker interface {
	Validate(ctx context.Context, token string) (userID string, platformID int32, err error)
}

// JoinedGroup is a group the user still belongs to.
type JoinedGroup struct {
	ID    string
	Super bool
}

// GroupDirectory reads group membership. It may be nil when the group service is down.
type GroupDirectory interface {
	MemberUserIDs(ctx context.Context, groupID, userID string) ([]string, error)
	Joined(ctx context.Context, userID string) ([]JoinedGroup, error)
}

// DismissedMembers removes every member after the dismiss notice has been pushed.
type DismissedMembers interface {
	ClearDismissedMembers(ctx context.Context, groupID, opUserID string) error
}

// JoinedGroups is the group service's joined-group incremental log and the full page behind it.
type JoinedGroups interface {
	IncrementalJoined(ctx context.Context, userID, versionID string, version uint64) (*grouppb.GetIncrementalJoinGroupsResp, error)
	ListJoined(ctx context.Context, userID string) ([]*grouppb.GroupInfo, error)
}

// Messages is the msg service. The gateway checks the token, then forwards message work here.
type Messages interface {
	SendMsg(ctx context.Context, msg *msgpb.MsgData) (*msgpb.SendMsgResp, error)
	GetMaxSeq(ctx context.Context, userID string) (*msgpb.GetMaxSeqResp, error)
	PullMessageBySeqs(ctx context.Context, req *msgpb.PullMessageBySeqsReq) (*msgpb.PushMessages, error)
	GetSeqMessage(ctx context.Context, req *msgpb.GetSeqMessageReq) (*msgpb.PushMessages, error)
	GetConversationsHasReadAndMaxSeq(ctx context.Context, req *msgpb.GetConversationsHasReadAndMaxSeqReq) (*msgpb.GetConversationsHasReadAndMaxSeqResp, error)
	GetLastMessage(ctx context.Context, req *msgpb.GetLastMessageReq) (*msgpb.GetLastMessageResp, error)
	MarkMsgsAsRead(ctx context.Context, req *msgpb.MarkMsgsAsReadReq) (*msgpb.MarkMsgsAsReadResp, error)
	MarkConversationAsRead(ctx context.Context, req *msgpb.MarkConversationAsReadReq) (*msgpb.MarkConversationAsReadResp, error)
	SetConversationHasReadSeq(ctx context.Context, req *msgpb.SetConversationHasReadSeqReq) (*msgpb.SetConversationHasReadSeqResp, error)
}

// Server accepts SDK websocket connections and pushes messages that arrive on toPush.
type Server struct {
	gatewaypb.UnimplementedMsgGatewayServer
	hub       *Hub
	tokens    TokenChecker
	groups    GroupDirectory
	joined    JoinedGroups
	syncLog   GroupSyncCache
	messages  Messages
	calls     *call.Board
	livekit   livekit.Config
	relations Relations
	maxConn   int64
	hooks     Hooks
	online    *online.Status
	upgrader  websocket.Upgrader
}

// Hooks carries the gateway webhook client and the online status callbacks.
type Hooks struct {
	Client       *webhook.Client
	AfterOnline  webhook.AfterConfig
	AfterOffline webhook.AfterConfig
	AfterKickOff webhook.AfterConfig
}

type Options struct {
	Tokens           TokenChecker
	Groups           GroupDirectory
	Joined           JoinedGroups
	GroupSync        GroupSyncCache
	Messages         Messages
	Calls            *call.Board
	LiveKit          livekit.Config
	Relations        Relations
	Online           *online.Status
	MaxConn          int64
	HandshakeTimeout time.Duration
	Hooks            Hooks
}

func New(opts Options) *Server {
	if opts.MaxConn <= 0 {
		opts.MaxConn = 100000
	}
	if opts.HandshakeTimeout <= 0 {
		opts.HandshakeTimeout = 10 * time.Second
	}
	return &Server{
		hub:       NewHub(),
		tokens:    opts.Tokens,
		groups:    opts.Groups,
		joined:    opts.Joined,
		syncLog:   opts.GroupSync,
		messages:  opts.Messages,
		calls:     opts.Calls,
		livekit:   opts.LiveKit,
		relations: opts.Relations,
		online:    opts.Online,
		maxConn:   opts.MaxConn,
		hooks:     opts.Hooks,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: opts.HandshakeTimeout,
			CheckOrigin:      func(*http.Request) bool { return true },
		},
	}
}

// SignCallToken signs a microphone-only LiveKit join token for one participant.
func (s *Server) SignCallToken(identity, room string) (string, string, error) {
	if s == nil {
		return "", "", fmt.Errorf("livekit is not configured")
	}
	return s.livekit.Sign(identity, room, time.Now())
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	info, code, msg := parseDial(r)
	if code != 0 {
		s.failDial(w, r, info.SendResponse, http.StatusBadRequest, code, msg)
		return
	}
	if s.hub.ConnCount() >= int(s.maxConn) {
		s.failDial(w, r, info.SendResponse, http.StatusServiceUnavailable, errInternal, "too many connections")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	userID, platformID, err := s.tokens.Validate(ctx, info.Token)
	cancel()
	if err != nil || userID == "" || userID != info.UserID || int(platformID) != info.PlatformID {
		s.failDial(w, r, info.SendResponse, http.StatusUnauthorized, errToken, "token invalid")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("websocket upgrade", "err", err, "userID", info.UserID)
		return
	}
	// 1. isMsgResp 时，升级成功后先回一帧空的成功 JSON。
	if info.SendResponse {
		if err := conn.WriteMessage(websocket.TextMessage, dialOK); err != nil {
			_ = conn.Close()
			return
		}
	}
	client := &Client{
		id:         newConnID(),
		userID:     info.UserID,
		platformID: info.PlatformID,
		token:      info.Token,
		compress:   info.Compression == "gzip",
		background: info.Background,
		sdkVersion: info.SDKVersion,
		encoder:    newEncoder(info.SDKType),
		conn:       newSocket(conn),
		server:     s,
	}
	slog.Debug("websocket connected", "userID", client.userID, "platformID", client.platformID, "sdkVersion", client.sdkVersion)
	quiet, kicked := s.hub.Add(client)
	for _, old := range quiet {
		old.close()
	}
	for _, old := range kicked {
		old.Kick()
	}
	go client.readLoop()
	s.report(client.userID, []int32{int32(client.platformID)}, nil)
	s.notifyOnline(client.userID)
	s.afterUserOnline(client)
}

// Deliver pushes one sequenced message to the online members of its conversation.
func (s *Server) Deliver(ctx context.Context, msg *msgpb.MsgData, conversationID string) {
	if msg == nil {
		return
	}
	if conversationID == "" {
		conversationID = msgconv.QueueKey(msg)
	}
	body, err := proto.Marshal(pushBody(msg, conversationID))
	if err != nil {
		slog.Error("marshal push", "err", err)
		return
	}
	for _, userID := range s.recipients(ctx, msg) {
		for _, client := range s.hub.Clients(userID) {
			// 2. 后台中的 iOS 不收在线推送，留给离线通知。
			if !acceptsOnlinePush(client) {
				continue
			}
			_ = client.write(Resp{ReqIdentifier: reqPushMsg, Data: body})
		}
	}
	// 成员名单已经取走，再删已解散群的成员行。只处理 1511 的通知副本。
	s.clearDismissedMembers(ctx, msg, conversationID)
}

func (s *Server) clearDismissedMembers(ctx context.Context, msg *msgpb.MsgData, conversationID string) {
	if msg == nil || msg.ContentType != 1511 || msg.GroupId == "" || msg.SendId == "" || !msgconv.IsNotificationConversation(conversationID) {
		return
	}
	cleaner, ok := s.groups.(DismissedMembers)
	if !ok {
		return
	}
	if err := cleaner.ClearDismissedMembers(ctx, msg.GroupId, msg.SendId); err != nil {
		slog.Warn("clear dismissed members", "err", err, "groupID", msg.GroupId)
	}
}

func acceptsOnlinePush(c *Client) bool {
	return c != nil && !(c.platformID == platformIOS && c.background)
}

func (s *Server) recipients(ctx context.Context, msg *msgpb.MsgData) []string {
	if msg.SessionType == 2 || msg.SessionType == 3 {
		if s.groups != nil && msg.GroupId != "" {
			ids, err := s.groups.MemberUserIDs(ctx, msg.GroupId, msg.SendId)
			if err == nil && len(ids) > 0 {
				return uniqueIDs(ids)
			}
			slog.Warn("group members", "err", err, "groupID", msg.GroupId)
		}
	}
	return uniqueIDs([]string{msg.SendId, msg.RecvId})
}

func (s *Server) notifyOnline(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.pushOnline(userID, s.platforms(ctx, userID))
}

func (s *Server) GetUsersOnlineStatus(ctx context.Context, req *gatewaypb.GetUsersOnlineStatusReq) (*gatewaypb.GetUsersOnlineStatusResp, error) {
	var ids []string
	if req != nil {
		ids = req.UserIds
	}
	if s.online != nil {
		users, err := s.onlineUsers(ctx, ids)
		if err != nil {
			return nil, err
		}
		return &gatewaypb.GetUsersOnlineStatusResp{Users: users}, nil
	}
	users := s.hub.Users(ids)
	resp := &gatewaypb.GetUsersOnlineStatusResp{}
	for userID, clients := range users {
		item := &gatewaypb.OnlineUser{UserId: userID, Status: onlineStatus}
		for _, client := range clients {
			item.Platforms = append(item.Platforms, &gatewaypb.Platform{
				PlatformId:   int32(client.platformID),
				ConnId:       client.id,
				IsBackground: client.background,
			})
		}
		resp.Users = append(resp.Users, item)
	}
	return resp, nil
}

func pushBody(msg *msgpb.MsgData, conversationID string) *msgpb.PushMessages {
	pull := &msgpb.PullMsgs{Msgs: []*msgpb.MsgData{msg}}
	out := &msgpb.PushMessages{}
	if msgconv.IsNotificationConversation(conversationID) {
		out.NotificationMsgs = map[string]*msgpb.PullMsgs{conversationID: pull}
	} else {
		out.Msgs = map[string]*msgpb.PullMsgs{conversationID: pull}
	}
	return out
}

type dialInfo struct {
	Token        string `json:"token"`
	UserID       string `json:"userID"`
	SendID       string `json:"sendID"`
	PlatformID   int    `json:"platformID"`
	OperationID  string `json:"operationID"`
	Compression  string `json:"compression"`
	SDKType      string `json:"sdkType"`
	SDKVersion   string `json:"sdkVersion"`
	Background   bool   `json:"background"`
	SendResponse bool   `json:"sendResponse"`
}

var dialOK = []byte(`{"errCode":0,"errMsg":"","errDlt":""}`)

func parseDial(r *http.Request) (dialInfo, int, string) {
	var info dialInfo
	query := r.URL.Query()
	queryResp := false
	if query.Get("isMsgResp") != "" {
		queryResp, _ = strconv.ParseBool(query.Get("isMsgResp"))
	}
	querySDK := query.Get("sdkVersion")
	if raw := query.Get("v"); raw != "" {
		body, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			info.SendResponse = queryResp
			return info, errArgs, "handshake is not base64"
		}
		if err := json.Unmarshal(body, &info); err != nil {
			info.SendResponse = queryResp
			return info, errArgs, "handshake is not json"
		}
	} else {
		info.Token = query.Get("token")
		info.UserID = query.Get("sendID")
		info.OperationID = query.Get("operationID")
		info.Compression = query.Get("compression")
		info.SDKType = query.Get("sdkType")
		info.SDKVersion = querySDK
		info.SendResponse = queryResp
		platformID, err := strconv.Atoi(query.Get("platformID"))
		if err != nil {
			info.SendResponse = queryResp
			return info, errArgs, "platformID is invalid"
		}
		info.PlatformID = platformID
		if query.Get("isBackground") != "" {
			info.Background, _ = strconv.ParseBool(query.Get("isBackground"))
		}
	}
	info.SendResponse = info.SendResponse || queryResp
	if info.SDKVersion == "" {
		info.SDKVersion = querySDK
	}
	if info.UserID == "" {
		info.UserID = info.SendID
	}
	if info.SDKType == "" {
		info.SDKType = sdkGo
	}
	if info.OperationID == "" || info.Token == "" || info.UserID == "" {
		return info, errArgs, "operationID, token and userID are required"
	}
	if info.PlatformID < 1 || info.PlatformID > 12 {
		return info, errArgs, "platformID is invalid"
	}
	if info.SDKType != sdkGo && info.SDKType != sdkJS {
		return info, errArgs, "sdkType is invalid"
	}
	return info, 0, ""
}

func (s *Server) failDial(w http.ResponseWriter, r *http.Request, send bool, status, code int, msg string) {
	if !send {
		writeErr(w, status, code, msg)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	body, err := json.Marshal(map[string]any{"errCode": code, "errMsg": msg, "errDlt": ""})
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, body)
}

func writeErr(w http.ResponseWriter, status, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"errCode": code, "errMsg": msg, "errDlt": ""})
}

func uniqueIDs(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

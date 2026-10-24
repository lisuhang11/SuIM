package ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/msgconv"
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
	MemberUserIDs(ctx context.Context, groupID string) ([]string, error)
	Joined(ctx context.Context, userID string) ([]JoinedGroup, error)
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
}

// Server accepts SDK websocket connections and pushes messages that arrive on toPush.
type Server struct {
	gatewaypb.UnimplementedMsgGatewayServer
	hub      *Hub
	tokens   TokenChecker
	groups   GroupDirectory
	joined   JoinedGroups
	syncLog  GroupSyncCache
	messages Messages
	maxConn  int64
	upgrader websocket.Upgrader
}

type Options struct {
	Tokens           TokenChecker
	Groups           GroupDirectory
	Joined           JoinedGroups
	GroupSync        GroupSyncCache
	Messages         Messages
	MaxConn          int64
	HandshakeTimeout time.Duration
}

func New(opts Options) *Server {
	if opts.MaxConn <= 0 {
		opts.MaxConn = 100000
	}
	if opts.HandshakeTimeout <= 0 {
		opts.HandshakeTimeout = 10 * time.Second
	}
	return &Server{
		hub:      NewHub(),
		tokens:   opts.Tokens,
		groups:   opts.Groups,
		joined:   opts.Joined,
		syncLog:  opts.GroupSync,
		messages: opts.Messages,
		maxConn:  opts.MaxConn,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: opts.HandshakeTimeout,
			CheckOrigin:      func(*http.Request) bool { return true },
		},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	info, code, msg := parseDial(r)
	if code != 0 {
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	if s.hub.ConnCount() >= int(s.maxConn) {
		writeErr(w, http.StatusServiceUnavailable, errInternal, "too many connections")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	userID, platformID, err := s.tokens.Validate(ctx, info.Token)
	cancel()
	if err != nil || userID == "" || userID != info.UserID || int(platformID) != info.PlatformID {
		writeErr(w, http.StatusUnauthorized, errToken, "token invalid")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("websocket upgrade", "err", err, "userID", info.UserID)
		return
	}
	client := &Client{
		id:         newConnID(),
		userID:     info.UserID,
		platformID: info.PlatformID,
		token:      info.Token,
		compress:   info.Compression == "gzip",
		background: info.Background,
		encoder:    newEncoder(info.SDKType),
		conn:       newSocket(conn),
		server:     s,
	}
	quiet, kicked := s.hub.Add(client)
	for _, old := range quiet {
		old.close()
	}
	for _, old := range kicked {
		old.Kick()
	}
	go client.readLoop()
	s.notifyOnline(client.userID)
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
			_ = client.write(Resp{ReqIdentifier: reqPushMsg, Data: body})
		}
	}
}

func (s *Server) recipients(ctx context.Context, msg *msgpb.MsgData) []string {
	if msg.SessionType == 2 || msg.SessionType == 3 {
		if s.groups != nil && msg.GroupId != "" {
			ids, err := s.groups.MemberUserIDs(ctx, msg.GroupId)
			if err == nil && len(ids) > 0 {
				return uniqueIDs(ids)
			}
			slog.Warn("group members", "err", err, "groupID", msg.GroupId)
		}
	}
	return uniqueIDs([]string{msg.SendId, msg.RecvId})
}

func (s *Server) notifyOnline(userID string) {
	body, err := proto.Marshal(&msgpb.SubUserOnlineStatusTips{Subscribers: []*msgpb.SubUserOnlineStatusElem{{
		UserId:            userID,
		OnlinePlatformIds: s.hub.Platforms(userID),
	}}})
	if err != nil {
		return
	}
	for _, watcher := range s.hub.Watchers(userID) {
		_ = watcher.write(Resp{ReqIdentifier: reqSubUserOnline, Data: body})
	}
}

func (s *Server) GetUsersOnlineStatus(_ context.Context, req *gatewaypb.GetUsersOnlineStatusReq) (*gatewaypb.GetUsersOnlineStatusResp, error) {
	var ids []string
	if req != nil {
		ids = req.UserIds
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
	Token       string `json:"token"`
	UserID      string `json:"userID"`
	SendID      string `json:"sendID"`
	PlatformID  int    `json:"platformID"`
	OperationID string `json:"operationID"`
	Compression string `json:"compression"`
	SDKType     string `json:"sdkType"`
	Background  bool   `json:"background"`
}

func parseDial(r *http.Request) (dialInfo, int, string) {
	var info dialInfo
	query := r.URL.Query()
	if raw := query.Get("v"); raw != "" {
		body, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return info, errArgs, "handshake is not base64"
		}
		if err := json.Unmarshal(body, &info); err != nil {
			return info, errArgs, "handshake is not json"
		}
	} else {
		info.Token = query.Get("token")
		info.UserID = query.Get("sendID")
		info.OperationID = query.Get("operationID")
		info.Compression = query.Get("compression")
		info.SDKType = query.Get("sdkType")
		platformID, err := strconv.Atoi(query.Get("platformID"))
		if err != nil {
			return info, errArgs, "platformID is invalid"
		}
		info.PlatformID = platformID
		if query.Get("isBackground") != "" {
			info.Background, _ = strconv.ParseBool(query.Get("isBackground"))
		}
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

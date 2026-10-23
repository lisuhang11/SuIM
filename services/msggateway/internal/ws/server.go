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

	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
	"github.com/lisuhang11/SuIM/services/msggateway/internal/history"
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

// Relations checks friendship and blocks before a single chat is accepted.
type Relations interface {
	IsFriend(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
	IsBlack(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
}

// HistoryReader loads sequenced messages from MySQL.
type HistoryReader interface {
	ConversationIDs(ctx context.Context, userID string) ([]string, error)
	MaxSeqs(ctx context.Context, conversationIDs []string) (map[string]int64, error)
	SeqState(ctx context.Context, userID string, conversationIDs []string) (map[string]history.SeqState, error)
	Range(ctx context.Context, conversationID string, begin, end, limit int64) ([]*msgpb.MsgData, error)
	BySeqs(ctx context.Context, conversationID string, seqs []int64) ([]*msgpb.MsgData, error)
	Last(ctx context.Context, conversationID string) (*msgpb.MsgData, error)
}

// Server accepts SDK websocket connections and pushes messages that arrive on toPush.
type Server struct {
	gatewaypb.UnimplementedMsgGatewayServer
	hub       *Hub
	tokens    TokenChecker
	groups    GroupDirectory
	relations Relations
	history   HistoryReader
	publish   mq.Producer
	maxConn   int64
	upgrader  websocket.Upgrader
}

type Options struct {
	Tokens           TokenChecker
	Groups           GroupDirectory
	Relations        Relations
	History          HistoryReader
	Publish          mq.Producer
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
		hub:       NewHub(),
		tokens:    opts.Tokens,
		groups:    opts.Groups,
		relations: opts.Relations,
		history:   opts.History,
		publish:   opts.Publish,
		maxConn:   opts.MaxConn,
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

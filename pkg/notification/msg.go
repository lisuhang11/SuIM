package notification

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"SuIM/pkg/notification/common_user"
	"SuIM/proto/sdkws"

	"github.com/google/uuid"
)

// NotificationSender 是通知发送的通用内核，与具体业务无关。
// 各业务服务通过嵌入 + 添加领域方法来扩展。
// 命名由来：通知最终都"落为一条消息"（sdkws.MsgData）经 msggateway 发出。
type NotificationSender struct {
	sessionTypeConf map[int32]int32 // contentType → sessionType 硬编码映射

	// pushMsg 是发送出口，通过依赖注入传入（通常是 msggateway.OnlinePushMsg）。
	pushMsg func(ctx context.Context, recvID string, msg *sdkws.MsgData) error

	// getUserInfo 可选：用户资料获取函数，用于填充 MsgData 的发送者昵称/头像。
	getUserInfo func(ctx context.Context, userID string) (common_user.CommonUser, error)
}

// Option 函数选项，用于构造 NotificationSender。
type Option func(*NotificationSender)

// WithPushMsg 注入消息推送函数（通常是 msggateway gRPC OnlinePushMsg 的封装）。
func WithPushMsg(fn func(ctx context.Context, recvID string, msg *sdkws.MsgData) error) Option {
	return func(s *NotificationSender) { s.pushMsg = fn }
}

// WithUserInfoFunc 注入用户资料获取函数（可选）。
func WithUserInfoFunc(fn func(ctx context.Context, userID string) (common_user.CommonUser, error)) Option {
	return func(s *NotificationSender) { s.getUserInfo = fn }
}

// SetUserInfoFunc 允许子嵌入类型在构造后注入用户资料获取函数。
func (s *NotificationSender) SetUserInfoFunc(fn func(ctx context.Context, userID string) (common_user.CommonUser, error)) {
	s.getUserInfo = fn
}

// NewNotificationSender 创建通知发送器内核实例。
func NewNotificationSender(opts ...Option) *NotificationSender {
	ns := &NotificationSender{
		sessionTypeConf: defaultSessionTypeConf(),
	}
	for _, o := range opts {
		o(ns)
	}
	return ns
}

// Notification 是各模块的统一入口：sendID 发给 recvID，内容为 contentType + payload（任意可 JSON 序列化的值）。
// 内部异步执行（goroutine），绝不阻塞业务主流程。
func (s *NotificationSender) Notification(ctx context.Context, sendID, recvID string, contentType int32, payload any) {
	sessionType := s.sessionTypeConf[contentType]
	go s.send(ctx, sendID, recvID, contentType, sessionType, payload)
}

// send 组装 sdkws.MsgData 并通过注入的 pushMsg 发送。
func (s *NotificationSender) send(ctx context.Context, sendID, recvID string, contentType, sessionType int32, payload any) {
	// 独立 context，不受主请求生命周期影响；5s 超时兜底。
	ctx = context.WithoutCancel(ctx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 1. 序列化 payload → JSON
	contentJSON, err := json.Marshal(payload)
	if err != nil {
		slog.Error("[notification] marshal payload failed",
			"type", contentType, "error", err)
		return
	}

	// 2. 组装 MsgData
	msg := &sdkws.MsgData{
		ClientMsgId: uuid.NewString(),
		SendId:      sendID,
		RecvId:      recvID,
		MsgFrom:     MsgFromSystem,
		ContentType: contentType,
		SessionType: sessionType,
		Content:     string(contentJSON),
		SendTime:    time.Now().UnixMilli(),
	}

	// 3. 可选：填充发送者昵称/头像
	if s.getUserInfo != nil {
		if user, err := s.getUserInfo(ctx, sendID); err == nil {
			msg.SenderNickname = user.GetNickname()
			msg.SenderFaceUrl = user.GetFaceURL()
		}
	}

	// 4. 调用注入的发送出口
	if s.pushMsg == nil {
		slog.Warn("[notification] pushMsg not configured, dropping",
			"recv_id", recvID, "type", contentType)
		return
	}
	if err := s.pushMsg(ctx, recvID, msg); err != nil {
		slog.Error("[notification] push failed",
			"recv_id", recvID, "type", contentType, "error", err)
		return
	}

	slog.Info("[notification] pushed",
		"recv_id", recvID, "type", contentType)
}

package client

import (
	"context"
	"encoding/json"

	"rtc/internal/types"

	pkgnotif "SuIM/pkg/notification"
	messagepb "SuIM/proto/messagepb"
	msggatewaypb "SuIM/proto/msggatewaypb"
	pushpb "SuIM/proto/pushpb"
	relationpb "SuIM/proto/relationpb"

	"google.golang.org/grpc/metadata"
)

func withForwardedMD(ctx context.Context) context.Context {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		return metadata.NewOutgoingContext(ctx, md.Copy())
	}
	return ctx
}

// RelationFriendChecker 通过 relation 服务校验双向好友。
type RelationFriendChecker struct {
	Client relationpb.RelationServiceClient
}

func (c *RelationFriendChecker) IsMutualFriend(ctx context.Context, user1, user2 string) (bool, error) {
	resp, err := c.Client.IsFriend(withForwardedMD(ctx), &relationpb.IsFriendReq{
		User1: user1,
		User2: user2,
	})
	if err != nil {
		return false, err
	}
	return resp.GetInUser1Friends() && resp.GetInUser2Friends(), nil
}

// MsgGatewayPresence 通过 msggateway 查询在线状态。
type MsgGatewayPresence struct {
	Client msggatewaypb.MsgGatewayClient
}

func (p *MsgGatewayPresence) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	resp, err := p.Client.GetUsersOnlineStatus(withForwardedMD(ctx), &msggatewaypb.GetUsersOnlineStatusReq{
		UserIds: []string{userID},
	})
	if err != nil {
		return false, err
	}
	for _, result := range resp.GetSuccessResult() {
		if result.GetUserId() == userID && result.GetStatus() == 1 {
			return true, nil
		}
	}
	return false, nil
}

// MessageTimelineWriter 通过 message 服务写入通话时间线摘要。
type MessageTimelineWriter struct {
	Client messagepb.MessageClient
}

func (w *MessageTimelineWriter) WriteCallTimeline(ctx context.Context, call *types.Call) error {
	tips := pkgnotif.CallTips{
		CallID:         call.CallID,
		CallerID:       call.CallerID,
		CalleeID:       call.CalleeID,
		MediaType:      call.MediaType,
		ConversationID: call.ConversationID,
		Reason:         call.EndReason,
		DurationSec:    call.DurationSec,
	}
	content, err := json.Marshal(tips)
	if err != nil {
		return err
	}
	_, err = w.Client.SendMsg(withForwardedMD(ctx), &messagepb.SendMsgReq{
		MsgData: &messagepb.MsgData{
			ClientMsgId:    "call_" + call.CallID,
			SendId:         call.CallerID,
			RecvId:         call.CalleeID,
			ConversationId: call.ConversationID,
			SessionType:    pkgnotif.SessionTypeSingle,
			ContentType:    pkgnotif.CallRecordContentType,
			Content:        string(content),
			MsgFrom:        pkgnotif.MsgFromSystem,
		},
	})
	return err
}

// PushOfflinePusher 调用 push 服务离线推送钩子（A 期可为 no-op）。
type PushOfflinePusher struct {
	Client pushpb.PushMsgServiceClient
}

func (p *PushOfflinePusher) PushCallSignal(ctx context.Context, call *types.Call, action string, userIDs ...string) error {
	if len(userIDs) == 0 {
		return nil
	}
	signalInfo, _ := json.Marshal(map[string]string{
		"call_id":    call.CallID,
		"media_type": call.MediaType,
		"action":     action,
	})
	_, err := p.Client.PushMsg(withForwardedMD(ctx), &pushpb.PushMsgReq{
		MsgData: &messagepb.MsgData{
			OfflinePush: &messagepb.OfflinePush{
				SignalInfo: string(signalInfo),
			},
		},
		ConversationId: call.ConversationID,
		UserIds:        userIDs,
	})
	return err
}

package logic

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
	svcconfig "github.com/lisuhang11/SuIM/services/rtc/internal/config"
)

// 私聊呼叫前检查好友和拉黑。
type Relations interface {
	IsFriend(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
	IsBlack(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
}

// 查询用户是否还在这个群里。
type Groups interface {
	InGroup(ctx context.Context, groupID, userID string) (bool, error)
}

// 保存通话状态。msggateway 只转发长连接帧。
type Service struct {
	rtcpb.UnimplementedRtcServiceServer
	calls          *call.Board
	relations      Relations
	groups         Groups
	media          livekit.Config
	agents         svcconfig.AgentsConfig
	agentsDispatch AgentDispatcher
	rdb            redis.UniversalClient
}

// 打开群语音。未注入时群邀请会被拒绝。
func (s *Service) UseGroups(groups Groups) {
	if s != nil {
		s.groups = groups
	}
}

// 用通话状态、好友检查和 LiveKit 配置构造通话服务。
func New(calls *call.Board, relations Relations, media livekit.Config) *Service {
	return &Service{calls: calls, relations: relations, media: media}
}

// UseRedis enables reading per-user agent voice preferences.
func (s *Service) UseRedis(rdb redis.UniversalClient) {
	if s != nil {
		s.rdb = rdb
	}
}

func (s *Service) preferredVoiceID(ctx context.Context, userID string) string {
	userID = strings.TrimSpace(userID)
	if s == nil || s.rdb == nil || userID == "" {
		return ""
	}
	val, err := s.rdb.Get(ctx, AgentVoiceKeyPrefix+userID).Result()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

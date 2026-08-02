// Package service 实现 1v1 通话状态机、LiveKit token、tip 与时间线写入。
package service

import (
	"context"
	"sync"
	"time"

	"rtc/internal/config"
	apperrors "rtc/internal/errors"
	lk "rtc/internal/livekit"
	"rtc/internal/logger"
	rtcnotif "rtc/internal/notification"
	"rtc/internal/types"
	"rtc/internal/types/interfaces"

	pkgnotif "SuIM/pkg/notification"

	"github.com/google/uuid"
)

type callService struct {
	repo             interfaces.CallRepository
	notifier         *rtcnotif.CallNotificationSender
	friends          friendChecker
	presence         presenceChecker
	timeline         timelineWriter
	offline          offlinePusher
	liveKitURL       string
	liveKitAPIKey    string
	liveKitAPISecret string
	ringTimeoutSec   int
	timers           sync.Map
}

// NewCallService 创建通话服务实例。
func NewCallService(
	repo interfaces.CallRepository,
	cfg *config.Config,
	notifier *rtcnotif.CallNotificationSender,
	friends friendChecker,
	presence presenceChecker,
	timeline timelineWriter,
	offline offlinePusher,
) interfaces.CallService {
	return &callService{
		repo:             repo,
		notifier:         notifier,
		friends:          friends,
		presence:         presence,
		timeline:         timeline,
		offline:          offline,
		liveKitURL:       cfg.LiveKitURL,
		liveKitAPIKey:    cfg.LiveKitAPIKey,
		liveKitAPISecret: cfg.LiveKitAPISecret,
		ringTimeoutSec:   cfg.RingTimeoutSec,
	}
}

func singleChatID(userA, userB string) string {
	a, b := userA, userB
	if a > b {
		a, b = b, a
	}
	return "si_" + a + "_" + b
}

func isParticipant(call *types.Call, userID string) bool {
	return call.CallerID == userID || call.CalleeID == userID
}

func callTipsFrom(call *types.Call) pkgnotif.CallTips {
	return pkgnotif.CallTips{
		CallID:         call.CallID,
		CallerID:       call.CallerID,
		CalleeID:       call.CalleeID,
		MediaType:      call.MediaType,
		ConversationID: call.ConversationID,
		Reason:         call.EndReason,
		DurationSec:    call.DurationSec,
	}
}

func (s *callService) isMutualFriend(ctx context.Context, user1, user2 string) (bool, error) {
	if s.friends == nil {
		return false, apperrors.NewInternalError("friend checker unavailable")
	}
	return s.friends.IsMutualFriend(ctx, user1, user2)
}

func (s *callService) isUserOnline(ctx context.Context, userID string) (bool, error) {
	if s.presence == nil {
		return false, apperrors.NewInternalError("presence checker unavailable")
	}
	return s.presence.IsUserOnline(ctx, userID)
}

func (s *callService) issueToken(roomName, userID string) (string, error) {
	return lk.NewJoinToken(s.liveKitAPIKey, s.liveKitAPISecret, roomName, userID)
}

func (s *callService) writeTimeline(ctx context.Context, call *types.Call) {
	if s.timeline == nil {
		return
	}
	if err := s.timeline.WriteCallTimeline(ctx, call); err != nil {
		logger.Error(ctx, "write call timeline failed", "call_id", call.CallID, "error", err)
	}
}

func (s *callService) pushOfflineSignal(ctx context.Context, call *types.Call, action string, userIDs ...string) {
	if s.offline == nil {
		return
	}
	if err := s.offline.PushCallSignal(ctx, call, action, userIDs...); err != nil {
		logger.Warn(ctx, "push offline signal failed", "call_id", call.CallID, "error", err)
	}
}

func (s *callService) createEndedCall(ctx context.Context, callerID, calleeID, mediaType, reason string) (*types.Call, error) {
	now := time.Now().UnixMilli()
	callID := uuid.NewString()
	call := &types.Call{
		CallID:         callID,
		ConversationID: singleChatID(callerID, calleeID),
		CallerID:       callerID,
		CalleeID:       calleeID,
		MediaType:      mediaType,
		Status:         types.CallStatusEnded,
		EndReason:      reason,
		RoomName:       "call_" + callID,
		StartedAt:      now,
		EndedAt:        now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.Create(ctx, call); err != nil {
		return nil, apperrors.NewInternalError("failed to create call record").WithDetails(err)
	}
	s.writeTimeline(ctx, call)
	return call, nil
}

func (s *callService) endCall(ctx context.Context, call *types.Call, reason string, answered bool) error {
	now := time.Now().UnixMilli()
	call.Status = types.CallStatusEnded
	call.EndReason = reason
	call.EndedAt = now
	call.UpdatedAt = now
	if answered && call.AnsweredAt > 0 {
		call.DurationSec = int32((now - call.AnsweredAt) / 1000)
	}
	s.stopRingTimeout(call.CallID)
	if err := s.repo.Update(ctx, call); err != nil {
		return apperrors.NewInternalError("failed to update call").WithDetails(err)
	}
	s.writeTimeline(ctx, call)
	return nil
}

func (s *callService) startRingTimeout(callID string) {
	timer := time.AfterFunc(time.Duration(s.ringTimeoutSec)*time.Second, func() {
		s.handleTimeout(context.Background(), callID)
	})
	s.timers.Store(callID, timer)
}

func (s *callService) stopRingTimeout(callID string) {
	if v, ok := s.timers.LoadAndDelete(callID); ok {
		v.(*time.Timer).Stop()
	}
}

func (s *callService) handleTimeout(ctx context.Context, callID string) {
	call, err := s.repo.GetByID(ctx, callID)
	if err != nil || call == nil {
		return
	}
	if call.Status != types.CallStatusRinging {
		return
	}
	if err := s.endCall(ctx, call, types.EndReasonTimeout, false); err != nil {
		logger.Error(ctx, "timeout end call failed", "call_id", callID, "error", err)
		return
	}
	tips := callTipsFrom(call)
	s.notifier.CallTimeout(ctx, call.CallerID, call.CalleeID, tips)
	s.notifier.CallTimeout(ctx, call.CalleeID, call.CallerID, tips)
}

func (s *callService) Invite(ctx context.Context, callerID, calleeID, mediaType string) (*types.Call, string, error) {
	if mediaType == "" {
		mediaType = types.MediaTypeAudio
	}
	if mediaType == types.MediaTypeVideo {
		return nil, "", apperrors.NewValidationError("video calls are not supported in phase A")
	}
	if mediaType != types.MediaTypeAudio {
		return nil, "", apperrors.NewValidationError("invalid media_type")
	}
	if callerID == "" || calleeID == "" {
		return nil, "", apperrors.NewValidationError("caller_id and callee_id are required")
	}
	if callerID == calleeID {
		return nil, "", apperrors.NewValidationError("cannot call yourself")
	}

	ok, err := s.isMutualFriend(ctx, callerID, calleeID)
	if err != nil {
		return nil, "", apperrors.NewInternalError("failed to check friendship").WithDetails(err)
	}
	if !ok {
		return nil, "", apperrors.NewNotFriendError()
	}

	online, err := s.isUserOnline(ctx, calleeID)
	if err != nil {
		return nil, "", apperrors.NewInternalError("failed to check online status").WithDetails(err)
	}
	if !online {
		if _, err := s.createEndedCall(ctx, callerID, calleeID, mediaType, types.EndReasonUnavailable); err != nil {
			return nil, "", err
		}
		return nil, "", apperrors.NewUnavailableError()
	}

	if active, err := s.repo.FindActiveByUser(ctx, callerID); err != nil {
		return nil, "", apperrors.NewInternalError("failed to check caller busy state").WithDetails(err)
	} else if active != nil {
		return nil, "", apperrors.NewBusyError()
	}
	if active, err := s.repo.FindActiveByUser(ctx, calleeID); err != nil {
		return nil, "", apperrors.NewInternalError("failed to check callee busy state").WithDetails(err)
	} else if active != nil {
		busyCall, err := s.createEndedCall(ctx, callerID, calleeID, mediaType, types.EndReasonBusy)
		if err != nil {
			return nil, "", err
		}
		tips := callTipsFrom(busyCall)
		s.notifier.CallBusy(ctx, calleeID, callerID, tips)
		return nil, "", apperrors.NewBusyError()
	}

	now := time.Now().UnixMilli()
	callID := uuid.NewString()
	call := &types.Call{
		CallID:         callID,
		ConversationID: singleChatID(callerID, calleeID),
		CallerID:       callerID,
		CalleeID:       calleeID,
		MediaType:      mediaType,
		Status:         types.CallStatusRinging,
		RoomName:       "call_" + callID,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.Create(ctx, call); err != nil {
		return nil, "", apperrors.NewInternalError("failed to create call").WithDetails(err)
	}

	token, err := s.issueToken(call.RoomName, callerID)
	if err != nil {
		return nil, "", apperrors.NewInternalError("failed to issue livekit token").WithDetails(err)
	}

	tips := callTipsFrom(call)
	s.notifier.CallInvite(ctx, callerID, calleeID, tips)
	s.startRingTimeout(callID)
	s.pushOfflineSignal(ctx, call, "invite", calleeID)

	return call, token, nil
}

func (s *callService) Accept(ctx context.Context, userID, callID string) (*types.Call, string, error) {
	call, err := s.getParticipantCall(ctx, userID, callID)
	if err != nil {
		return nil, "", err
	}
	if call.CalleeID != userID {
		return nil, "", apperrors.NewNotParticipantError()
	}
	if call.Status != types.CallStatusRinging {
		return nil, "", apperrors.NewInvalidStateError("call is not ringing")
	}

	now := time.Now().UnixMilli()
	call.Status = types.CallStatusAccepted
	call.AnsweredAt = now
	call.UpdatedAt = now
	s.stopRingTimeout(callID)
	if err := s.repo.Update(ctx, call); err != nil {
		return nil, "", apperrors.NewInternalError("failed to update call").WithDetails(err)
	}

	token, err := s.issueToken(call.RoomName, userID)
	if err != nil {
		return nil, "", apperrors.NewInternalError("failed to issue livekit token").WithDetails(err)
	}

	tips := callTipsFrom(call)
	s.notifier.CallAccepted(ctx, userID, call.CallerID, tips)
	return call, token, nil
}

func (s *callService) Reject(ctx context.Context, userID, callID string) (*types.Call, error) {
	call, err := s.getParticipantCall(ctx, userID, callID)
	if err != nil {
		return nil, err
	}
	if call.CalleeID != userID {
		return nil, apperrors.NewNotParticipantError()
	}
	if call.Status != types.CallStatusRinging {
		return nil, apperrors.NewInvalidStateError("call is not ringing")
	}
	if err := s.endCall(ctx, call, types.EndReasonRejected, false); err != nil {
		return nil, err
	}
	tips := callTipsFrom(call)
	s.notifier.CallRejected(ctx, userID, call.CallerID, tips)
	return call, nil
}

func (s *callService) Cancel(ctx context.Context, userID, callID string) (*types.Call, error) {
	call, err := s.getParticipantCall(ctx, userID, callID)
	if err != nil {
		return nil, err
	}
	if call.CallerID != userID {
		return nil, apperrors.NewNotParticipantError()
	}
	if call.Status != types.CallStatusRinging {
		return nil, apperrors.NewInvalidStateError("call is not ringing")
	}
	if err := s.endCall(ctx, call, types.EndReasonCancelled, false); err != nil {
		return nil, err
	}
	tips := callTipsFrom(call)
	s.notifier.CallCancelled(ctx, userID, call.CalleeID, tips)
	return call, nil
}

func (s *callService) Hangup(ctx context.Context, userID, callID string) (*types.Call, error) {
	call, err := s.getParticipantCall(ctx, userID, callID)
	if err != nil {
		return nil, err
	}
	if call.Status != types.CallStatusAccepted && call.Status != types.CallStatusActive {
		return nil, apperrors.NewInvalidStateError("call is not in progress")
	}
	answered := call.AnsweredAt > 0
	if err := s.endCall(ctx, call, types.EndReasonCompleted, answered); err != nil {
		return nil, err
	}
	tips := callTipsFrom(call)
	s.notifier.CallEnded(ctx, userID, call.CallerID, tips)
	s.notifier.CallEnded(ctx, userID, call.CalleeID, tips)
	s.pushOfflineSignal(ctx, call, "ended", call.CallerID, call.CalleeID)
	return call, nil
}

func (s *callService) GetCall(ctx context.Context, userID, callID string) (*types.Call, error) {
	return s.getParticipantCall(ctx, userID, callID)
}

func (s *callService) RefreshToken(ctx context.Context, userID, callID string) (string, string, error) {
	call, err := s.getParticipantCall(ctx, userID, callID)
	if err != nil {
		return "", "", err
	}
	if call.Status == types.CallStatusEnded {
		return "", "", apperrors.NewInvalidStateError("call has ended")
	}
	token, err := s.issueToken(call.RoomName, userID)
	if err != nil {
		return "", "", apperrors.NewInternalError("failed to issue livekit token").WithDetails(err)
	}
	return token, call.RoomName, nil
}

func (s *callService) getParticipantCall(ctx context.Context, userID, callID string) (*types.Call, error) {
	call, err := s.repo.GetByID(ctx, callID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get call").WithDetails(err)
	}
	if call == nil {
		return nil, apperrors.NewNotFoundError("call not found")
	}
	if !isParticipant(call, userID) {
		return nil, apperrors.NewNotParticipantError()
	}
	return call, nil
}

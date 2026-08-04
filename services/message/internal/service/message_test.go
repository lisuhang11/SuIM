package service

import (
	"context"
	"testing"

	apperrors "message/internal/errors"
	"message/internal/types"

	pkgnotif "SuIM/pkg/notification"
)

type repositoryStub struct {
	readUserID       string
	readConversation string
	readSeq          int64
	deleteUserID     string
	deleteConv       string
	deleteSeqs       []int64
	seqUsers         []types.SeqUser
	listUserID       string
	listConvIDs      []string
	sendCalled       int
	revokeMsg        *types.Message
	revokeErr        error
	bySeqMsgs        []types.Message
	lastMsgs         map[string]types.Message
}

type tipNotifierStub struct {
	revokeCalls []struct {
		revoker string
		recvIDs []string
		sess    int32
		tips    pkgnotif.RevokeMsgTips
	}
	readCalls []struct {
		reader string
		recvID string
		sess   int32
		tips   pkgnotif.MarkAsReadTips
	}
}

func (t *tipNotifierStub) RevokeNotification(_ context.Context, revokerUserID string, recvIDs []string, sessionType int32, tips pkgnotif.RevokeMsgTips) {
	t.revokeCalls = append(t.revokeCalls, struct {
		revoker string
		recvIDs []string
		sess    int32
		tips    pkgnotif.RevokeMsgTips
	}{revokerUserID, append([]string(nil), recvIDs...), sessionType, tips})
}

func (t *tipNotifierStub) HasReadReceiptNotification(_ context.Context, readerUserID, recvID string, sessionType int32, tips pkgnotif.MarkAsReadTips) {
	t.readCalls = append(t.readCalls, struct {
		reader string
		recvID string
		sess   int32
		tips   pkgnotif.MarkAsReadTips
	}{readerUserID, recvID, sessionType, tips})
}

func (r *repositoryStub) SendMessage(context.Context, *types.Message, []string) error {
	r.sendCalled++
	return nil
}

// blackCheckerStub 模拟 relation.IsBlack：blocked 表示 recv 是否拉黑了 send。
type blackCheckerStub struct {
	blocked bool
	err     error
	sendID  string
	recvID  string
}

func (b *blackCheckerStub) IsBlockedByPeer(_ context.Context, sendID, recvID string) (bool, error) {
	b.sendID, b.recvID = sendID, recvID
	return b.blocked, b.err
}

type groupMemberResolverStub struct {
	ids []string
	err error
}

func (g *groupMemberResolverStub) GetGroupMemberUserIDs(context.Context, string) ([]string, error) {
	return append([]string(nil), g.ids...), g.err
}

// friendCheckerStub 模拟双向好友校验。
type friendCheckerStub struct {
	ok  bool
	err error
}

func (f *friendCheckerStub) IsMutualFriend(context.Context, string, string) (bool, error) {
	return f.ok, f.err
}
func (*repositoryStub) MapSendTimeByConvSeq(_ context.Context, conversationSeqs map[string]int64) (map[string]int64, error) {
	out := make(map[string]int64, len(conversationSeqs))
	for id, seq := range conversationSeqs {
		if seq > 0 {
			out[id] = seq * 1000 // deterministic stub send_time
		}
	}
	return out, nil
}
func (*repositoryStub) GetByClientMsgIDs(context.Context, string, []string) ([]types.Message, error) {
	return nil, nil
}
func (*repositoryStub) GetBySenderClientMsgIDs(context.Context, string, []string) ([]types.Message, error) {
	return nil, nil
}
func (r *repositoryStub) GetBySeqs(context.Context, string, string, []int64) ([]types.Message, error) {
	return append([]types.Message(nil), r.bySeqMsgs...), nil
}
func (*repositoryStub) GetHistory(context.Context, string, string, int64, int, int) ([]types.Message, int64, error) {
	return nil, 0, nil
}
func (r *repositoryStub) GetLastMessage(context.Context, string, []string) (map[string]types.Message, error) {
	if r.lastMsgs != nil {
		return r.lastMsgs, nil
	}
	return map[string]types.Message{}, nil
}
func (r *repositoryStub) Revoke(context.Context, string, string, string, int32, string) (*types.Message, error) {
	if r.revokeErr != nil {
		return nil, r.revokeErr
	}
	if r.revokeMsg != nil {
		return r.revokeMsg, nil
	}
	return &types.Message{}, nil
}
func (r *repositoryStub) SetReadSeq(_ context.Context, userID, conversationID string, seq int64) error {
	r.readUserID, r.readConversation, r.readSeq = userID, conversationID, seq
	return nil
}
func (r *repositoryStub) DeleteForUser(_ context.Context, userID, conversationID string, seqs []int64) error {
	r.deleteUserID, r.deleteConv = userID, conversationID
	r.deleteSeqs = append([]int64(nil), seqs...)
	return nil
}
func (r *repositoryStub) ListSeqUser(_ context.Context, userID string, conversationIDs []string) ([]types.SeqUser, error) {
	r.listUserID = userID
	r.listConvIDs = append([]string(nil), conversationIDs...)
	var out []types.SeqUser
	for _, row := range r.seqUsers {
		if row.UserID != userID {
			continue
		}
		if len(conversationIDs) > 0 {
			found := false
			for _, cid := range conversationIDs {
				if row.ConversationID == cid {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func TestMarkMsgsAsReadUsesUserReadCursor(t *testing.T) {
	repo := &repositoryStub{}
	svc := &messageService{repo: repo}
	if err := svc.MarkMsgsAsRead(context.Background(), "conv-1", "user-1", 42); err != nil {
		t.Fatal(err)
	}
	if repo.readUserID != "user-1" || repo.readConversation != "conv-1" || repo.readSeq != 42 {
		t.Fatalf("unexpected read cursor: user=%q conversation=%q seq=%d", repo.readUserID, repo.readConversation, repo.readSeq)
	}
}

func TestRevokeMsgPushesTipToBothParties(t *testing.T) {
	repo := &repositoryStub{revokeMsg: &types.Message{
		ConversationID: "si_a_b",
		MsgDataModel: types.MsgDataModel{
			SendID: "user-a", RecvID: "user-b", ClientMsgID: "c1",
			SessionType: types.SessionTypeSingle, Seq: 9,
		},
		RevokeModel: types.RevokeModel{RevokeTime: 123, RevokeUserID: "user-a"},
	}}
	tips := &tipNotifierStub{}
	svc := &messageService{repo: repo, tips: tips}
	if err := svc.RevokeMsg(context.Background(), "si_a_b", "c1", "user-a", 0, ""); err != nil {
		t.Fatal(err)
	}
	if len(tips.revokeCalls) != 1 {
		t.Fatalf("revoke tip calls=%d", len(tips.revokeCalls))
	}
	call := tips.revokeCalls[0]
	if call.tips.ClientMsgID != "c1" || call.tips.Seq != 9 || call.tips.ConversationID != "si_a_b" {
		t.Fatalf("tips=%#v", call.tips)
	}
	if len(call.recvIDs) != 2 {
		t.Fatalf("recvIDs=%v", call.recvIDs)
	}
}

func TestRevokeMsgRejectsWhenExpired(t *testing.T) {
	repo := &repositoryStub{revokeErr: ErrRevokeExpired}
	svc := &messageService{repo: repo, tips: &tipNotifierStub{}}
	err := svc.RevokeMsg(context.Background(), "si_a_b", "c1", "user-a", 0, "")
	if err == nil {
		t.Fatal("expected expire error")
	}
	ae := apperrors.GetAppError(err)
	if ae == nil || ae.Code != apperrors.CodeRevokeExpired {
		t.Fatalf("got %#v", err)
	}
}

func TestRevokeMsgPushesTipToGroupMembers(t *testing.T) {
	repo := &repositoryStub{revokeMsg: &types.Message{
		ConversationID: "gid_g1",
		MsgDataModel: types.MsgDataModel{
			SendID: "user-a", GroupID: "g1", ClientMsgID: "cg1",
			SessionType: types.SessionTypeGroup, Seq: 3,
		},
		RevokeModel: types.RevokeModel{RevokeTime: 1},
	}}
	tips := &tipNotifierStub{}
	svc := &messageService{
		repo:         repo,
		tips:         tips,
		groupMembers: &groupMemberResolverStub{ids: []string{"user-a", "user-b", "user-c"}},
	}
	if err := svc.RevokeMsg(context.Background(), "gid_g1", "cg1", "user-a", 0, ""); err != nil {
		t.Fatal(err)
	}
	if len(tips.revokeCalls) != 1 || len(tips.revokeCalls[0].recvIDs) != 3 {
		t.Fatalf("call=%#v", tips.revokeCalls)
	}
}

func TestMarkMsgsAsReadPushesTipToPeer(t *testing.T) {
	repo := &repositoryStub{
		bySeqMsgs: []types.Message{{
			ConversationID: "si_a_b",
			MsgDataModel: types.MsgDataModel{
				SendID: "user-a", RecvID: "user-b",
				SessionType: types.SessionTypeSingle, Seq: 42,
			},
		}},
	}
	tips := &tipNotifierStub{}
	svc := &messageService{repo: repo, tips: tips}
	if err := svc.MarkMsgsAsRead(context.Background(), "si_a_b", "user-b", 42); err != nil {
		t.Fatal(err)
	}
	if len(tips.readCalls) != 1 {
		t.Fatalf("read tip calls=%d", len(tips.readCalls))
	}
	call := tips.readCalls[0]
	if call.recvID != "user-a" || call.tips.HasReadSeq != 42 || call.tips.MarkAsReadUserID != "user-b" {
		t.Fatalf("call=%#v", call)
	}
}

func TestMarkMsgsAsReadGroupPushesTipToSelf(t *testing.T) {
	repo := &repositoryStub{
		bySeqMsgs: []types.Message{{
			ConversationID: "gid_g1",
			MsgDataModel: types.MsgDataModel{
				SendID: "user-a", GroupID: "g1",
				SessionType: types.SessionTypeGroup, Seq: 10,
			},
		}},
	}
	tips := &tipNotifierStub{}
	svc := &messageService{repo: repo, tips: tips}
	if err := svc.MarkMsgsAsRead(context.Background(), "gid_g1", "user-b", 10); err != nil {
		t.Fatal(err)
	}
	if len(tips.readCalls) != 1 || tips.readCalls[0].recvID != "user-b" {
		t.Fatalf("call=%#v", tips.readCalls)
	}
}

func TestGetMaxSeqReturnsOnlyCallerRows(t *testing.T) {
	repo := &repositoryStub{seqUsers: []types.SeqUser{
		{UserID: "u1", ConversationID: "c1", MaxSeq: 10, MinSeq: 2, ReadSeq: 3},
		{UserID: "u2", ConversationID: "c2", MaxSeq: 99, ReadSeq: 1},
	}}
	svc := &messageService{repo: repo}
	got, err := svc.GetMaxSeq(context.Background(), "u1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["c1"].MaxSeq != 10 || got["c1"].MinSeq != 2 {
		t.Fatalf("got %#v", got)
	}
	if repo.listUserID != "u1" {
		t.Fatalf("list user = %q", repo.listUserID)
	}
}

func TestGetConversationsHasReadAndMaxSeqFilter(t *testing.T) {
	repo := &repositoryStub{seqUsers: []types.SeqUser{
		{UserID: "u1", ConversationID: "c1", MaxSeq: 10, ReadSeq: 3},
		{UserID: "u1", ConversationID: "c2", MaxSeq: 5, ReadSeq: 5},
	}}
	svc := &messageService{repo: repo}
	got, err := svc.GetConversationsHasReadAndMaxSeq(context.Background(), "u1", []string{"c2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["c2"].MaxSeq != 5 || got["c2"].HasReadSeq != 5 || got["c2"].MaxSeqTime != 5000 {
		t.Fatalf("got %#v", got)
	}
}

func TestGetActiveConversationSortAndLimit(t *testing.T) {
	repo := &repositoryStub{seqUsers: []types.SeqUser{
		{UserID: "u1", ConversationID: "c1", MaxSeq: 10, ReadSeq: 3},
		{UserID: "u1", ConversationID: "c2", MaxSeq: 5, ReadSeq: 5},
	}}
	svc := &messageService{repo: repo}
	got, err := svc.GetActiveConversation(context.Background(), "u1", []string{"c1", "c2"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ConversationID != "c1" || got[0].LastTime != 10000 {
		t.Fatalf("got %#v", got)
	}
}

func TestGetMaxSeqRequiresUserID(t *testing.T) {
	svc := &messageService{repo: &repositoryStub{}}
	if _, err := svc.GetMaxSeq(context.Background(), "", nil); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDeleteMsgsIsScopedToUser(t *testing.T) {
	repo := &repositoryStub{}
	svc := &messageService{repo: repo}
	if err := svc.DeleteMsgs(context.Background(), "user-1", "conv-1", []int64{2, 3}); err != nil {
		t.Fatal(err)
	}
	if repo.deleteUserID != "user-1" || repo.deleteConv != "conv-1" || len(repo.deleteSeqs) != 2 {
		t.Fatalf("unexpected delete scope: user=%q conversation=%q seqs=%v", repo.deleteUserID, repo.deleteConv, repo.deleteSeqs)
	}
}

func TestSendMsgRejectsWhenBlockedByPeer(t *testing.T) {
	repo := &repositoryStub{}
	checker := &blackCheckerStub{blocked: true}
	svc := &messageService{
		repo:          repo,
		blackChecker:  checker,
		friendChecker: &friendCheckerStub{ok: true},
	}

	_, err := svc.SendMsg(context.Background(), &types.Message{
		ConversationID: "si_a_b",
		MsgDataModel: types.MsgDataModel{
			SendID:      "user-b",
			RecvID:      "user-a",
			ClientMsgID: "c1",
			SessionType: types.SessionTypeSingle,
			ContentType: 101,
			Content:     "hi",
		},
	})
	if err == nil {
		t.Fatal("expected blocked-by-peer error")
	}
	ae := apperrors.GetAppError(err)
	if ae == nil || ae.Code != apperrors.CodeBlockedByPeer {
		t.Fatalf("got error %#v", err)
	}
	if repo.sendCalled != 0 {
		t.Fatalf("message should not be persisted, sendCalled=%d", repo.sendCalled)
	}
	if checker.sendID != "user-b" || checker.recvID != "user-a" {
		t.Fatalf("IsBlockedByPeer args = (%q, %q)", checker.sendID, checker.recvID)
	}
}

func TestSendMsgAllowsWhenNotBlocked(t *testing.T) {
	repo := &repositoryStub{}
	svc := &messageService{
		repo:          repo,
		blackChecker:  &blackCheckerStub{blocked: false},
		friendChecker: &friendCheckerStub{ok: true},
	}

	got, err := svc.SendMsg(context.Background(), &types.Message{
		ConversationID: "si_a_b",
		MsgDataModel: types.MsgDataModel{
			SendID:      "user-b",
			RecvID:      "user-a",
			ClientMsgID: "c2",
			SessionType: types.SessionTypeSingle,
			ContentType: 101,
			Content:     "hi",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || repo.sendCalled != 1 {
		t.Fatalf("expected persist once, got=%v sendCalled=%d", got, repo.sendCalled)
	}
}

func TestSendMsgRejectsWhenNotFriend(t *testing.T) {
	repo := &repositoryStub{}
	svc := &messageService{
		repo:          repo,
		friendChecker: &friendCheckerStub{ok: false},
		blackChecker:  &blackCheckerStub{blocked: false},
	}

	_, err := svc.SendMsg(context.Background(), &types.Message{
		ConversationID: "si_a_b",
		MsgDataModel: types.MsgDataModel{
			SendID:      "user-b",
			RecvID:      "user-a",
			ClientMsgID: "c-not-friend",
			SessionType: types.SessionTypeSingle,
			ContentType: 101,
			Content:     "hi",
		},
	})
	if err == nil {
		t.Fatal("expected not-friend error")
	}
	ae := apperrors.GetAppError(err)
	if ae == nil || ae.Code != apperrors.CodeNotFriend {
		t.Fatalf("got error %#v", err)
	}
	if repo.sendCalled != 0 {
		t.Fatalf("message should not be persisted, sendCalled=%d", repo.sendCalled)
	}
}

func TestSendMsgRejectsWhenSenderNotGroupMember(t *testing.T) {
	repo := &repositoryStub{}
	svc := &messageService{
		repo:         repo,
		groupMembers: &groupMemberResolverStub{ids: []string{"user-a", "user-c"}},
	}

	_, err := svc.SendMsg(context.Background(), &types.Message{
		ConversationID: "gid_g1",
		MsgDataModel: types.MsgDataModel{
			SendID:      "user-b",
			GroupID:     "g1",
			ClientMsgID: "c-not-member",
			SessionType: types.SessionTypeGroup,
			ContentType: 101,
			Content:     "hi",
		},
	})
	if err == nil {
		t.Fatal("expected not-group-member error")
	}
	ae := apperrors.GetAppError(err)
	if ae == nil || ae.Code != apperrors.CodeNotGroupMember {
		t.Fatalf("got error %#v", err)
	}
	if repo.sendCalled != 0 {
		t.Fatalf("message should not be persisted, sendCalled=%d", repo.sendCalled)
	}
}

func TestSendMsgSkipsBlackCheckForGroup(t *testing.T) {
	repo := &repositoryStub{}
	checker := &blackCheckerStub{blocked: true}
	svc := &messageService{
		repo:         repo,
		blackChecker: checker,
		groupMembers: &groupMemberResolverStub{ids: []string{"user-a", "user-b", "user-c"}},
	}

	got, err := svc.SendMsg(context.Background(), &types.Message{
		ConversationID: "gid_g1",
		MsgDataModel: types.MsgDataModel{
			SendID:      "user-b",
			RecvID:      "user-a",
			GroupID:     "g1",
			ClientMsgID: "c3",
			SessionType: types.SessionTypeGroup,
			ContentType: 101,
			Content:     "hi",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if checker.sendID != "" {
		t.Fatal("group chat must not call black checker")
	}
	if repo.sendCalled != 1 {
		t.Fatalf("sendCalled=%d", repo.sendCalled)
	}
	if got == nil || len(got.RecvUserIDs) != 2 {
		t.Fatalf("expected server-resolved recipients excluding sender, got %#v", got)
	}
}

func TestSendMsgResolvesGroupMembersFromConversationID(t *testing.T) {
	repo := &repositoryStub{}
	resolver := &groupMemberResolverStub{ids: []string{"owner", "member"}}
	svc := &messageService{repo: repo, groupMembers: resolver}

	got, err := svc.SendMsg(context.Background(), &types.Message{
		ConversationID: "gid_abc",
		MsgDataModel: types.MsgDataModel{
			SendID:      "owner",
			ClientMsgID: "c4",
			SessionType: types.SessionTypeGroup,
			ContentType: 101,
			Content:     "hi",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.GroupID != "abc" {
		t.Fatalf("groupID=%q", got.GroupID)
	}
	if len(got.RecvUserIDs) != 1 || got.RecvUserIDs[0] != "member" {
		t.Fatalf("RecvUserIDs=%v", got.RecvUserIDs)
	}
}

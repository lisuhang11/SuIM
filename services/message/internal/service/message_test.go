package service

import (
	"context"
	"testing"

	"message/internal/types"
)

type repositoryStub struct {
	readUserID       string
	readConversation string
	readSeq          int64
	deleteUserID     string
	deleteConv       string
	deleteSeqs       []int64
}

func (*repositoryStub) SendMessage(context.Context, *types.Message, []string) error { return nil }
func (*repositoryStub) GetByClientMsgIDs(context.Context, string, []string) ([]types.Message, error) {
	return nil, nil
}
func (*repositoryStub) GetBySenderClientMsgIDs(context.Context, string, []string) ([]types.Message, error) {
	return nil, nil
}
func (*repositoryStub) GetBySeqs(context.Context, string, string, []int64) ([]types.Message, error) {
	return nil, nil
}
func (*repositoryStub) GetHistory(context.Context, string, string, int64, int, int) ([]types.Message, int64, error) {
	return nil, 0, nil
}
func (*repositoryStub) Revoke(context.Context, string, string, string, int32, string) error {
	return nil
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

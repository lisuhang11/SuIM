package service

import (
	"context"
	"strings"
	"testing"

	apperrors "relation/internal/errors"
	"relation/internal/types"
)

type friendUpdateFakeRepo struct {
	exists      bool
	existsErr   error
	updateErr   error
	lastFields  map[string]any
	lastOwner   string
	lastFriend  string
	updateCalls int
}

func (f *friendUpdateFakeRepo) CreateFriendRequest(context.Context, *types.FriendRequest) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) GetFriendRequest(context.Context, string, string) (*types.FriendRequest, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) GetPendingBetween(context.Context, string, string) (*types.FriendRequest, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) ResetFriendRequestPending(context.Context, string, string, string) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) UpdateFriendRequestStatus(context.Context, string, string, string, types.FriendRequestHandleResult, string) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) AcceptFriendRequest(context.Context, string, string, string, string) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) ListIncomingRequests(context.Context, string, []int32, int, int) ([]*types.FriendRequest, int64, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) ListOutgoingRequests(context.Context, string, []int32, int, int) ([]*types.FriendRequest, int64, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) CountUnhandledRequests(context.Context, string) (int64, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) CreateFriend(context.Context, *types.Friend) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) DeleteFriendPair(context.Context, string, string) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) FriendExists(context.Context, string, string) (bool, error) {
	return f.exists, f.existsErr
}
func (f *friendUpdateFakeRepo) ListFriends(context.Context, string, int, int) ([]*types.Friend, int64, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) UpdateFriend(_ context.Context, ownerUserID, friendUserID string, fields map[string]any) error {
	f.updateCalls++
	f.lastOwner = ownerUserID
	f.lastFriend = friendUserID
	f.lastFields = fields
	return f.updateErr
}
func (f *friendUpdateFakeRepo) ListFriendUserIDs(context.Context, string) ([]string, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) ListFriendsByIDs(context.Context, string, []string) ([]*types.Friend, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) FindOwnerUserIDsWhoFriended(context.Context, string) ([]string, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) IncrVersion(context.Context, string, []string, int8, bool) error {
	return nil
}
func (f *friendUpdateFakeRepo) EnsureFriendVersion(context.Context, string) (*types.FriendVersion, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) GetFriendVersion(context.Context, string) (*types.FriendVersion, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) ListFriendVersionLogs(context.Context, string, uint64, uint64) ([]*types.FriendVersionLog, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) CreateBlock(context.Context, *types.Black) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) DeleteBlock(context.Context, string, string) error {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) BlockExists(context.Context, string, string) (bool, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) ListBlocks(context.Context, string, int, int) ([]*types.Black, int64, error) {
	panic("unexpected")
}
func (f *friendUpdateFakeRepo) FindBlock(context.Context, string, string) (*types.Black, error) {
	panic("unexpected")
}

func TestUpdateFriend(t *testing.T) {
	remark := "阿航"
	empty := ""
	pinned := true
	unpinned := false
	longRemark := strings.Repeat("备注", 33) // 66 runes

	tests := []struct {
		name      string
		exists    bool
		remark    *string
		isPinned  *bool
		wantCode  apperrors.ErrorCode
		wantField map[string]any
	}{
		{name: "remark only", exists: true, remark: &remark, wantField: map[string]any{"remark": "阿航"}},
		{name: "pin only", exists: true, isPinned: &pinned, wantField: map[string]any{"is_pinned": true}},
		{name: "clear remark", exists: true, remark: &empty, wantField: map[string]any{"remark": ""}},
		{name: "unpin", exists: true, isPinned: &unpinned, wantField: map[string]any{"is_pinned": false}},
		{name: "empty patch", exists: true, wantCode: apperrors.CodeValidation},
		{name: "not friend", exists: false, remark: &remark, wantCode: apperrors.CodeRelationNotFound},
		{name: "remark too long", exists: true, remark: &longRemark, wantCode: apperrors.CodeValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &friendUpdateFakeRepo{exists: tt.exists}
			svc := NewRelationService(repo, nil)
			err := svc.UpdateFriend(context.Background(), "u1", "u2", tt.remark, tt.isPinned)
			if tt.wantCode != 0 {
				ae := apperrors.GetAppError(err)
				if ae == nil || ae.Code != tt.wantCode {
					t.Fatalf("got err %v, want code %d", err, tt.wantCode)
				}
				if repo.updateCalls != 0 {
					t.Fatalf("expected no update call")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if repo.updateCalls != 1 || repo.lastOwner != "u1" || repo.lastFriend != "u2" {
				t.Fatalf("bad update call: %+v", repo)
			}
			for k, v := range tt.wantField {
				if repo.lastFields[k] != v {
					t.Fatalf("field %s: got %v want %v", k, repo.lastFields[k], v)
				}
			}
		})
	}
}

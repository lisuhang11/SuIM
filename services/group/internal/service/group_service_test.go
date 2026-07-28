package service

import (
	"context"
	"testing"
	"time"

	"group/internal/types"
	"group/internal/types/interfaces"
)

type testRepo struct {
	interfaces.GroupRepository
	group        *types.Group
	members      map[string]*types.GroupMember
	request      *types.GroupRequest
	transactions int
	updated      bool
	created      bool
}

func (r *testRepo) WithinTransaction(_ context.Context, fn func(interfaces.GroupRepository) error) error {
	r.transactions++
	return fn(r)
}
func (r *testRepo) GetGroup(context.Context, string) (*types.Group, error) { return r.group, nil }
func (r *testRepo) GetMember(_ context.Context, _, userID string) (*types.GroupMember, error) {
	return r.members[userID], nil
}
func (r *testRepo) UpdateMember(context.Context, *types.GroupMember) error {
	r.updated = true
	return nil
}
func (r *testRepo) MemberExists(_ context.Context, _, userID string) (bool, error) {
	_, ok := r.members[userID]
	return ok, nil
}
func (r *testRepo) GetRequest(context.Context, string, string) (*types.GroupRequest, error) {
	return r.request, nil
}
func (r *testRepo) UpdateRequest(_ context.Context, req *types.GroupRequest) error {
	r.request = req
	r.updated = true
	return nil
}
func (r *testRepo) CreateMember(_ context.Context, member *types.GroupMember) error {
	r.members[member.UserID] = member
	r.created = true
	return nil
}
func (r *testRepo) ListMembers(context.Context, string, int, int) ([]*types.GroupMember, int64, error) {
	result := make([]*types.GroupMember, 0, len(r.members))
	for _, member := range r.members {
		result = append(result, member)
	}
	return result, int64(len(result)), nil
}

type testVerifier struct{ interfaces.UserVerifier }

func (testVerifier) UserExists(context.Context, string) (bool, error) { return true, nil }

type testPublisher struct{ events []interfaces.GroupEvent }

func (p *testPublisher) Publish(_ context.Context, event interfaces.GroupEvent) error {
	p.events = append(p.events, event)
	return nil
}

func TestSetMemberMuteRejectsSameOrHigherRole(t *testing.T) {
	repo := &testRepo{group: &types.Group{GroupID: "g1"}, members: map[string]*types.GroupMember{
		"admin": {GroupID: "g1", UserID: "admin", RoleLevel: types.GroupMemberRoleAdmin},
		"owner": {GroupID: "g1", UserID: "owner", RoleLevel: types.GroupMemberRoleOwner},
	}}
	svc := NewGroupService(repo, testVerifier{})
	err := svc.SetMemberMute(context.Background(), "g1", "admin", "owner", time.Now().Add(time.Hour).UnixMilli())
	if err == nil {
		t.Fatal("expected permission error")
	}
	if repo.updated {
		t.Fatal("owner mute must not be persisted")
	}
	if repo.transactions != 1 {
		t.Fatalf("transactions = %d, want 1", repo.transactions)
	}
}

func TestDirectJoinReusesHandledRequestAndCreatesMemberAtomically(t *testing.T) {
	repo := &testRepo{
		group:   &types.Group{GroupID: "g1", NeedVerification: 0},
		members: map[string]*types.GroupMember{},
		request: &types.GroupRequest{GroupID: "g1", UserID: "user-1", HandleResult: -1},
	}
	publisher := &testPublisher{}
	svc := NewGroupService(repo, testVerifier{}, publisher)
	err := svc.ApplyToJoinGroup(context.Background(), &types.ApplyInput{GroupID: "g1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("ApplyToJoinGroup: %v", err)
	}
	if !repo.created || repo.request.HandleResult != 1 {
		t.Fatalf("created = %v, result = %d", repo.created, repo.request.HandleResult)
	}
	if repo.transactions != 1 {
		t.Fatalf("transactions = %d, want 1", repo.transactions)
	}
	if len(publisher.events) != 1 || publisher.events[0].Type != "group.members_joined" {
		t.Fatalf("events = %#v", publisher.events)
	}
}

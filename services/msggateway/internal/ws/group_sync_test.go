package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lisuhang11/SuIM/proto/group"
)

func TestApplyJoinedGroupsKeepsInsertAndDelete(t *testing.T) {
	current := []*grouppb.GroupInfo{{GroupId: "g1", GroupName: "old"}, {GroupId: "g2"}}
	got := applyJoinedGroups(current, []string{"g2"}, []*grouppb.GroupInfo{{GroupId: "g3", OwnerUserId: "a"}}, []*grouppb.GroupInfo{{GroupId: "g1", GroupName: "new"}})
	if len(got) != 2 || got[0].GetGroupId() != "g1" || got[0].GetGroupName() != "new" || got[1].GetGroupId() != "g3" {
		t.Fatalf("groups %+v", got)
	}
}

func TestSyncGroupsFullThenDelta(t *testing.T) {
	syncer := &fakeJoined{full: []*grouppb.GroupInfo{{GroupId: "g1", OwnerUserId: "a", MemberCount: 2}}}
	cache := &memGroupSync{}
	srv := New(Options{Tokens: fakeTokens{}, Joined: syncer, GroupSync: cache})
	body := postSync(t, srv)
	if body["code"] != float64(0) {
		t.Fatalf("body %+v", body)
	}
	groups := body["data"].(map[string]any)["groups"].([]any)
	if len(groups) != 1 || groups[0].(map[string]any)["group_id"] != "g1" {
		t.Fatalf("groups %+v", groups)
	}
	syncer.full = nil
	syncer.delta = &grouppb.GetIncrementalJoinGroupsResp{
		VersionId: "v2",
		Version:   2,
		Insert:    []*grouppb.GroupInfo{{GroupId: "g2", OwnerUserId: "b"}},
	}
	body = postSync(t, srv)
	groups = body["data"].(map[string]any)["groups"].([]any)
	if len(groups) != 2 || groups[1].(map[string]any)["owner_user_id"] != "b" {
		t.Fatalf("groups %+v", groups)
	}
}

func postSync(t *testing.T, srv *Server) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/msg/sync_groups", nil)
	req.Header.Set("token", "t")
	rr := httptest.NewRecorder()
	srv.HandleSyncGroups(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

type fakeJoined struct {
	full  []*grouppb.GroupInfo
	delta *grouppb.GetIncrementalJoinGroupsResp
	calls int
}

func (f *fakeJoined) IncrementalJoined(context.Context, string, string, uint64) (*grouppb.GetIncrementalJoinGroupsResp, error) {
	f.calls++
	if f.calls == 1 || f.delta == nil {
		return &grouppb.GetIncrementalJoinGroupsResp{VersionId: "v1", Version: 1, Full: true}, nil
	}
	return f.delta, nil
}

func (f *fakeJoined) ListJoined(context.Context, string) ([]*grouppb.GroupInfo, error) {
	return f.full, nil
}

type memGroupSync struct {
	rec GroupSyncRecord
	ok  bool
}

func (m *memGroupSync) Load(context.Context, string) (GroupSyncRecord, bool, error) {
	return m.rec, m.ok, nil
}

func (m *memGroupSync) Save(_ context.Context, _ string, rec GroupSyncRecord) error {
	m.rec = rec
	m.ok = true
	return nil
}

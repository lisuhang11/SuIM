package ws

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
)

const groupSyncKeyPrefix = "suim:msggateway:joined:"

// GroupSyncCache remembers the joined-group version this gateway has already applied.
type GroupSyncCache interface {
	Load(ctx context.Context, userID string) (GroupSyncRecord, bool, error)
	Save(ctx context.Context, userID string, rec GroupSyncRecord) error
}

// GroupSyncRecord is one user's joined groups at a version.
type GroupSyncRecord struct {
	VersionID string               `json:"version_id"`
	Version   uint64               `json:"version"`
	Groups    []*grouppb.GroupInfo `json:"groups"`
}

type redisGroupSync struct {
	rdb redis.UniversalClient
}

// NewRedisGroupSync stores the joined-group version log cursor in Redis.
func NewRedisGroupSync(rdb redis.UniversalClient) GroupSyncCache {
	return redisGroupSync{rdb: rdb}
}

func (s redisGroupSync) Load(ctx context.Context, userID string) (GroupSyncRecord, bool, error) {
	raw, err := s.rdb.Get(ctx, groupSyncKeyPrefix+userID).Bytes()
	if err == redis.Nil {
		return GroupSyncRecord{}, false, nil
	}
	if err != nil {
		return GroupSyncRecord{}, false, err
	}
	var rec GroupSyncRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return GroupSyncRecord{}, false, nil
	}
	return rec, true, nil
}

func (s redisGroupSync) Save(ctx context.Context, userID string, rec GroupSyncRecord) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, groupSyncKeyPrefix+userID, raw, 0).Err()
}

// HandleSyncGroups applies the joined-group incremental log and returns the current list.
func (s *Server) HandleSyncGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTTP(w, http.StatusMethodNotAllowed, int(errorcode.InvalidJSON), "method not allowed", nil)
		return
	}
	userID, _, ok := s.tokenUser(w, r)
	if !ok {
		return
	}
	groups, err := s.syncJoinedGroups(r.Context(), userID)
	if err != nil {
		status, code, text := httpFromErr(err)
		writeHTTP(w, status, code, text, nil)
		return
	}
	if groups == nil {
		groups = []*grouppb.GroupInfo{}
	}
	writeHTTP(w, http.StatusOK, 0, "", map[string]any{"groups": groups})
}

func (s *Server) syncJoinedGroups(ctx context.Context, userID string) ([]*grouppb.GroupInfo, error) {
	// 1. 没有群服务时不能凭空编一份列表。
	if s.joined == nil {
		return nil, errorcode.ErrInternal.WithMsg("group service is not configured")
	}
	var current GroupSyncRecord
	if s.syncLog != nil {
		loaded, ok, err := s.syncLog.Load(ctx, userID)
		if err != nil {
			return nil, err
		}
		if ok {
			current = loaded
		}
	}
	// 2. 用已应用的版本向群服务要增量。版本对不上时 full=true，资料要另分页。
	delta, err := s.joined.IncrementalJoined(ctx, userID, current.VersionID, current.Version)
	if err != nil {
		return nil, err
	}
	groups := current.Groups
	if delta.GetFull() || current.VersionID == "" {
		groups, err = s.joined.ListJoined(ctx, userID)
		if err != nil {
			return nil, err
		}
	} else {
		groups = applyJoinedGroups(groups, delta.GetDelete(), delta.GetInsert(), delta.GetUpdate())
	}
	// 3. 记下新版本，下次只拉差额。
	if s.syncLog != nil {
		if err := s.syncLog.Save(ctx, userID, GroupSyncRecord{
			VersionID: delta.GetVersionId(),
			Version:   delta.GetVersion(),
			Groups:    groups,
		}); err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func applyJoinedGroups(current []*grouppb.GroupInfo, deleted []string, insert, update []*grouppb.GroupInfo) []*grouppb.GroupInfo {
	removed := map[string]struct{}{}
	for _, id := range deleted {
		if id != "" {
			removed[id] = struct{}{}
		}
	}
	next := make([]*grouppb.GroupInfo, 0, len(current))
	index := map[string]int{}
	for _, group := range current {
		if group == nil || group.GetGroupId() == "" {
			continue
		}
		if _, drop := removed[group.GetGroupId()]; drop {
			continue
		}
		index[group.GetGroupId()] = len(next)
		next = append(next, group)
	}
	for _, group := range update {
		if group == nil || group.GetGroupId() == "" {
			continue
		}
		if at, ok := index[group.GetGroupId()]; ok {
			next[at] = group
			continue
		}
		index[group.GetGroupId()] = len(next)
		next = append(next, group)
	}
	for _, group := range insert {
		if group == nil || group.GetGroupId() == "" {
			continue
		}
		if _, ok := index[group.GetGroupId()]; ok {
			continue
		}
		index[group.GetGroupId()] = len(next)
		next = append(next, group)
	}
	return next
}

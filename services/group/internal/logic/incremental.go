package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

const (
	syncLimit        = 200
	batchMemberLimit = 100
)

func (s *Server) groupsByID(ctx context.Context, ids []string) ([]*grouppb.GroupInfo, error) {
	groups, err := s.store.FindGroups(ctx, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query groups")
	}
	out := make([]*grouppb.GroupInfo, 0, len(groups))
	for _, group := range groups {
		out = append(out, groupPB(group))
	}
	return out, nil
}

func (s *Server) membersByID(ctx context.Context, groupID string, ids []string) ([]*grouppb.GroupMemberFullInfo, error) {
	members, err := s.store.FindMembers(ctx, groupID, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query members")
	}
	if err := s.fillMembers(ctx, members); err != nil {
		return nil, err
	}
	out := make([]*grouppb.GroupMemberFullInfo, 0, len(members))
	for _, member := range members {
		out = append(out, memberPB(member))
	}
	return out, nil
}

func needFull(head model.VersionHead, versionID string, version uint64) bool {
	return version == 0 || versionID == "" || head.ID == "" || head.ID != versionID
}

func splitIDs(logs []model.VersionLog) (insertIDs, updateIDs, deleteIDs []string) {
	for _, log := range logs {
		if log.ElemID == "" || log.ElemID == model.SortElemID {
			continue
		}
		switch log.State {
		case model.VersionInsert:
			insertIDs = append(insertIDs, log.ElemID)
		case model.VersionUpdate:
			updateIDs = append(updateIDs, log.ElemID)
		case model.VersionDelete:
			deleteIDs = append(deleteIDs, log.ElemID)
		}
	}
	return insertIDs, updateIDs, deleteIDs
}

func splitMemberLogs(logs []model.VersionLog) (insertIDs, updateIDs, deleteIDs []string, groupChanged bool, sortVersion uint64) {
	for _, log := range logs {
		switch log.ElemID {
		case "":
			groupChanged = true
		case model.SortElemID:
			sortVersion = log.Version
		default:
			groupChanged = true
			switch log.State {
			case model.VersionInsert:
				insertIDs = append(insertIDs, log.ElemID)
			case model.VersionUpdate:
				updateIDs = append(updateIDs, log.ElemID)
			case model.VersionDelete:
				deleteIDs = append(deleteIDs, log.ElemID)
			}
		}
	}
	return insertIDs, updateIDs, deleteIDs, groupChanged, sortVersion
}

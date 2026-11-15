package logic

import (
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

type sessionPair struct {
	Access  *model.Token
	Refresh *model.Token
}

func (s *Server) issuePair(userID string, platformID int32, familyID, parentJTI string) (*sessionPair, error) {
	now := s.now().UTC()
	if familyID == "" {
		id, err := s.newID()
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("create family id")
		}
		familyID = id
	}
	accessID, err := s.newID()
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create access id")
	}
	refreshID, err := s.newID()
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create refresh id")
	}
	accessRaw, accessExp, err := s.issuer.SignAccess(accessID, userID, platformID, now)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("sign access")
	}
	refreshRaw, refreshExp, err := s.issuer.SignRefresh(refreshID, userID, platformID, now)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("sign refresh")
	}
	return &sessionPair{
		Access: &model.Token{
			ID:         accessID,
			UserID:     userID,
			PlatformID: platformID,
			FamilyID:   familyID,
			ParentJTI:  parentJTI,
			Raw:        accessRaw,
			Type:       model.TokenTypeAccess,
			ExpiresAt:  accessExp.UTC(),
		},
		Refresh: &model.Token{
			ID:         refreshID,
			UserID:     userID,
			PlatformID: platformID,
			FamilyID:   familyID,
			ParentJTI:  parentJTI,
			Raw:        refreshRaw,
			Type:       model.TokenTypeRefresh,
			ExpiresAt:  refreshExp.UTC(),
		},
	}, nil
}

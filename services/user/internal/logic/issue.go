package logic

import (
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

func (s *Server) issue(userID string, platformID int32) (*model.Token, error) {
	// 1. 生成令牌 ID 并签名。
	now := s.now().UTC()
	id, err := s.newID()
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create token id")
	}
	raw, exp, err := s.issuer.Sign(id, userID, platformID, now)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("sign token")
	}
	return &model.Token{
		ID:         id,
		UserID:     userID,
		PlatformID: platformID,
		Raw:        raw,
		Hash:       token.Hash(raw),
		Type:       model.TokenType,
		ExpiresAt:  exp.UTC(),
	}, nil
}

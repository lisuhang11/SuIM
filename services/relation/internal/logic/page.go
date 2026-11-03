package logic

import (
	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func page(offset, limit int32) (int, int, error) {
	if offset < 0 || limit < 0 || limit > 200 {
		return 0, 0, errorcode.ErrRelationInvalidPage
	}
	if limit == 0 {
		limit = 20
	}
	return int(offset), int(limit), nil
}

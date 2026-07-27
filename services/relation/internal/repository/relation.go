// Package repository 提供关系服务的数据访问层，按功能聚合好友请求、好友和拉黑操作。
//
// relationRepository 聚合了好友请求、好友和拉黑三类数据操作于一体，
// 遵循按功能域组织仓库的模式，而非每个表一个仓库。
// 存储细节封装在此层，服务层仅依赖 interfaces.RelationRepository。
package repository

import (
	"errors"

	"relation/internal/types/interfaces"

	"gorm.io/gorm"
)

var (
	// ErrFriendRequestNotFound 好友请求不存在错误。
	ErrFriendRequestNotFound = errors.New("friend request not found")
	// ErrFriendNotFound 好友记录不存在错误。
	ErrFriendNotFound = errors.New("friend not found")
	// ErrBlackNotFound 拉黑记录不存在错误。
	ErrBlackNotFound = errors.New("black record not found")
)

// relationRepository 是 interfaces.RelationRepository 的 GORM 实现。
// 它持有一个 *gorm.DB 并封装所有关系表操作。
type relationRepository struct {
	db *gorm.DB
}

// NewRelationRepository 创建按功能聚合的关系仓库。
// 方法分别实现在 relation_request.go、relation_friend.go、relation_block.go 中，
// 均属于同一个 relationRepository 类型。
func NewRelationRepository(db *gorm.DB) interfaces.RelationRepository {
	return &relationRepository{db: db}
}

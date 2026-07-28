// Package interfaces 定义关系服务的接口契约，解耦业务逻辑与具体实现。
package interfaces

import (
	"context"

	"relation/internal/types"
)

// RelationService 定义关系业务逻辑的接口契约。
type RelationService interface {
	// SendFriendRequest 发送好友请求。
	SendFriendRequest(ctx context.Context, fromUserID, toUserID, msg string) error

	// RespondFriendApply 响应好友请求（接受或拒绝），toUserID 必须等于 userID。
	// handleResult 为 types.FriendRequestAccepted 或 types.FriendRequestRejected。
	RespondFriendApply(ctx context.Context, fromUserID, toUserID, userID string, handleResult types.FriendRequestHandleResult, handleMsg string) error

	// DeleteFriend 删除两个用户之间的双向好友关系。
	DeleteFriend(ctx context.Context, userID, friendID string) error

	// GetFriends 分页获取用户的好友 ID 列表。
	GetFriends(ctx context.Context, userID string, offset, limit int) (friendIDs []string, total int, err error)

	// GetIncomingApplyTo 分页获取收到的好友请求，handleResults 为空则不过滤状态。
	GetIncomingApplyTo(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error)

	// GetOutgoingApplyFrom 分页获取发出的好友请求，handleResults 为空则不过滤状态。
	GetOutgoingApplyFrom(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error)

	// GetUnhandledApplyCount 获取发给指定用户的未处理好友请求数量。
	GetUnhandledApplyCount(ctx context.Context, userID string) (int64, error)

	// BlockUser 拉黑指定用户并移除已有好友关系。
	BlockUser(ctx context.Context, userID, blockedUserID string) error

	// UnblockUser 取消拉黑指定用户。
	UnblockUser(ctx context.Context, userID, blockedUserID string) error

	// GetBlockedUsers 分页获取已拉黑的用户 ID 列表。
	GetBlockedUsers(ctx context.Context, userID string, offset, limit int) (blockedIDs []string, total int, err error)

	// IsFriend 返回两个用户之间的双向好友关系：
	//   - inUser1Friends：user2 是否在 user1 的好友列表中
	//   - inUser2Friends：user1 是否在 user2 的好友列表中
	IsFriend(ctx context.Context, user1, user2 string) (inUser1Friends, inUser2Friends bool, err error)

	// IsBlack 返回两个用户之间的双向拉黑关系：
	//   - inUser1Blacklist：user1 是否拉黑了 user2
	//   - inUser2Blacklist：user2 是否拉黑了 user1
	IsBlack(ctx context.Context, user1, user2 string) (inUser1Blacklist, inUser2Blacklist bool, err error)
}

// RelationRepository 聚合所有关系数据访问（好友请求、好友、拉黑）于一个接口。
// 存储细节（表结构、SQL 语句）封装在此层，不得泄露到服务层。
type RelationRepository interface {
	// ----- 好友请求 -----
	// CreateFriendRequest 持久化新的好友请求。
	CreateFriendRequest(ctx context.Context, req *types.FriendRequest) error
	// GetFriendRequest 根据复合键查询好友请求，不存在则返回 ErrFriendRequestNotFound。
	GetFriendRequest(ctx context.Context, fromUserID, toUserID string) (*types.FriendRequest, error)
	// GetPendingBetween 查询两个用户之间任意方向的待处理请求，不存在则返回 ErrFriendRequestNotFound。
	GetPendingBetween(ctx context.Context, userA, userB string) (*types.FriendRequest, error)
	// UpdateFriendRequestStatus 更新好友请求的处理状态。
	UpdateFriendRequestStatus(ctx context.Context, fromUserID, toUserID, handlerUserID string, status types.FriendRequestHandleResult, handleMsg string) error
	// AcceptFriendRequest 接受好友请求并在同一事务中创建双向好友关系。
	AcceptFriendRequest(ctx context.Context, fromUserID, toUserID, handlerUserID, handleMsg string) error
	// ListIncomingRequests 分页查询发给指定用户的好友请求，按 handleResults 筛选状态（含总数）。
	ListIncomingRequests(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error)
	// ListOutgoingRequests 分页查询指定用户发出的好友请求，按 handleResults 筛选状态（含总数）。
	ListOutgoingRequests(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error)
	// CountUnhandledRequests 统计发给指定用户的未处理好友请求数量。
	CountUnhandledRequests(ctx context.Context, toUserID string) (int64, error)

	// ----- 好友 -----
	// CreateFriend 持久化单向好友记录。
	CreateFriend(ctx context.Context, f *types.Friend) error
	// DeleteFriendPair 删除两个用户间的双向好友关系。
	DeleteFriendPair(ctx context.Context, userA, userB string) error
	// FriendExists 判断 (owner, friend) 好友记录是否存在。
	FriendExists(ctx context.Context, ownerUserID, friendUserID string) (bool, error)
	// ListFriends 分页查询用户的好友列表（含总数）。
	ListFriends(ctx context.Context, ownerUserID string, offset, limit int) (friends []*types.Friend, total int64, err error)

	// ----- 拉黑 -----
	// CreateBlock 持久化单向拉黑记录。
	CreateBlock(ctx context.Context, b *types.Black) error
	// DeleteBlock 删除指定用户对目标用户的拉黑记录。
	DeleteBlock(ctx context.Context, ownerUserID, blockUserID string) error
	// BlockExists 判断 (owner, blocked) 拉黑记录是否存在。
	BlockExists(ctx context.Context, ownerUserID, blockUserID string) (bool, error)
	// ListBlocks 分页查询用户的拉黑列表（含总数）。
	ListBlocks(ctx context.Context, ownerUserID string, offset, limit int) (blocks []*types.Black, total int64, err error)
	// FindBlock 查询指定用户对目标用户的拉黑记录，不存在则返回 ErrBlackNotFound。
	FindBlock(ctx context.Context, ownerUserID, targetUserID string) (*types.Black, error)
}

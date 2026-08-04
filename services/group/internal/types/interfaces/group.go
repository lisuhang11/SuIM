// Package interfaces 定义群组服务的接口契约，解耦业务逻辑、数据访问和跨服务调用。
package interfaces

import (
	"context"

	"group/internal/types"
)

// UserVerifier 检查用户 ID 是否在 user 服务中存在。
// 作为跨服务边界保留在接口后面，使 group 服务在编译时不依赖 user 服务的内部实现。
type UserVerifier interface {
	// UserExists 判断单个用户是否存在。
	UserExists(ctx context.Context, userID string) (bool, error)
	// UsersExist 批量检查用户 ID 集合，返回每个 ID 的存在状态。
	UsersExist(ctx context.Context, userIDs []string) (map[string]bool, error)
	// Authenticate 校验访问令牌并返回令牌绑定的用户 ID。
	Authenticate(ctx context.Context, token string) (userID string, err error)
}

// GroupEvent 描述已提交的群领域变更，供会话和消息服务消费。
type GroupEvent struct {
	Type             string
	GroupID          string
	OperatorUserID   string
	SubjectUserIDs   []string
	RecipientUserIDs []string
}

// GroupEventPublisher 在数据库事务提交后联动会话和系统消息。
type GroupEventPublisher interface {
	Publish(ctx context.Context, event GroupEvent) error
}

// GroupService 定义群组业务逻辑的接口契约。
type GroupService interface {
	// ---- 群组生命周期 ----
	// CreateGroup 创建群组，创建者成为群主，可选邀请初始成员。
	CreateGroup(ctx context.Context, in *types.CreateGroupInput) (groupID string, group *types.Group, err error)
	// DismissGroup 硬删除群组及其成员和请求，opUserID 必须是群主。
	DismissGroup(ctx context.Context, groupID, opUserID string) error
	// TransferGroupOwner 转让群主，原群主降级为管理员。
	TransferGroupOwner(ctx context.Context, groupID, opUserID, newOwnerUserID string) error
	// UpdateGroupInfo 更新群组可修改字段，opUserID 必须是群主或管理员。
	UpdateGroupInfo(ctx context.Context, in *types.UpdateGroupInfoInput) (*types.Group, error)
	// GetGroup 根据 ID 获取群组信息。
	GetGroup(ctx context.Context, groupID string) (*types.Group, error)
	// GetGroupsInfo 批量获取群组信息（cache-aside）。
	GetGroupsInfo(ctx context.Context, groupIDs []string) ([]*types.Group, error)
	// CanManageGroup 校验用户是否为群主或管理员。
	CanManageGroup(ctx context.Context, groupID, userID string) error

	// ---- 成员管理 ----
	// InviteUserToGroup 邀请用户加入群组，opUserID 必须是群主或管理员。
	InviteUserToGroup(ctx context.Context, in *types.InviteInput) error
	// KickGroupMember 踢出群成员，opUserID 角色必须高于目标成员。
	KickGroupMember(ctx context.Context, groupID, opUserID, targetUserID string) error
	// QuitGroup 退出群组，群主需先转让后方可退出。
	QuitGroup(ctx context.Context, groupID, userID string) error
	// GetGroupMembers 分页获取群成员列表。
	GetGroupMembers(ctx context.Context, groupID, opUserID string, offset, limit int) (members []*types.GroupMember, total int, err error)
	// GetGroupMemberUserIDs 返回群全部成员 userID（对齐 OpenIM，供消息 fan-out）。
	GetGroupMemberUserIDs(ctx context.Context, groupID string) ([]string, error)
	// GetJoinedGroups 分页获取用户已加入的群组列表。
	GetJoinedGroups(ctx context.Context, userID string, offset, limit int) (groups []*types.Group, total int, err error)
	// GetIncrementalJoinGroup 按水位返回已加入群增量（或 Full）。
	GetIncrementalJoinGroup(ctx context.Context, userID, versionID string, version uint64) (*types.IncrementalJoinGroupsResult, error)
	// GetFullJoinGroupIDs 返回当前完整已加入群 ID 列表。
	GetFullJoinGroupIDs(ctx context.Context, userID string) ([]string, error)
	// GetIncrementalGroupMember 按水位返回单群成员增量（或 Full）。
	GetIncrementalGroupMember(ctx context.Context, groupID, opUserID, versionID string, version uint64) (*types.IncrementalGroupMembersResult, error)
	// GetFullGroupMemberUserIDs 返回单群完整成员 ID 列表及当前 version。
	GetFullGroupMemberUserIDs(ctx context.Context, groupID, opUserID string) (userIDs []string, version uint64, versionID string, err error)

	// ---- 禁言 ----
	// SetGroupMute 设置群全员禁言开关，opUserID 必须是群主或管理员。
	SetGroupMute(ctx context.Context, groupID, opUserID string, muted bool) error
	// SetMemberMute 设置单个成员的禁言到期时间（0 取消），opUserID 必须是群主或管理员。
	SetMemberMute(ctx context.Context, groupID, opUserID, targetUserID string, muteEndTime int64) error

	// ---- 入群申请 ----
	// ApplyToJoinGroup 提交入群申请，无需验证时自动批准。
	ApplyToJoinGroup(ctx context.Context, in *types.ApplyInput) error
	// GetPendingApplications 获取群组待处理申请（群主/管理员视角）。
	GetPendingApplications(ctx context.Context, groupID, opUserID string, offset, limit int) (requests []*types.GroupRequest, total int, err error)
	// GetUserApplications 获取用户的入群申请记录（申请人视角）。
	GetUserApplications(ctx context.Context, userID string, offset, limit int) (requests []*types.GroupRequest, total int, err error)
	// HandleApplication 处理入群申请（同意/拒绝），同意时添加成员。
	HandleApplication(ctx context.Context, in *types.HandleInput) error
	// GetUnhandledApplicationCount 统计群组待处理的入群申请数量。
	GetUnhandledApplicationCount(ctx context.Context, groupID, opUserID string) (int, error)
}

// GroupRepository 聚合所有群组数据访问（群组、成员、入群请求）于一个接口。
// 存储细节封装在此层，不得泄露到服务层。
type GroupRepository interface {
	// WithinTransaction 在同一个数据库事务中执行完整业务操作。
	WithinTransaction(ctx context.Context, fn func(GroupRepository) error) error
	// ----- 群组 -----
	// CreateGroup 持久化群组记录。
	CreateGroup(ctx context.Context, g *types.Group) error
	// GetGroup 根据 ID 获取群组，不存在则返回 ErrGroupNotFound。
	GetGroup(ctx context.Context, groupID string) (*types.Group, error)
	// UpdateGroup 写入所有群组字段（包括零值）。
	UpdateGroup(ctx context.Context, g *types.Group) error
	// DeleteGroup 硬删除群组。
	DeleteGroup(ctx context.Context, groupID string) error
	// ListGroupsByIDs 根据 ID 列表批量查询群组（不保证顺序）。
	ListGroupsByIDs(ctx context.Context, groupIDs []string) ([]*types.Group, error)

	// ----- 成员 -----
	// CreateMember 持久化群成员记录。
	CreateMember(ctx context.Context, m *types.GroupMember) error
	// GetMember 根据群组和用户 ID 查询成员，不存在则返回 ErrMemberNotFound。
	GetMember(ctx context.Context, groupID, userID string) (*types.GroupMember, error)
	// UpdateMember 写入所有成员字段（包括零值）。
	UpdateMember(ctx context.Context, m *types.GroupMember) error
	// DeleteMember 删除单个成员记录。
	DeleteMember(ctx context.Context, groupID, userID string) error
	// DeleteMembersByGroup 删除群组所有成员记录。
	DeleteMembersByGroup(ctx context.Context, groupID string) error
	// ListMembers 分页查询群成员列表（含总数）。
	ListMembers(ctx context.Context, groupID string, offset, limit int) (members []*types.GroupMember, total int64, err error)
	// MapGroupMemberNum 批量统计各群成员数（对齐 OpenIM MapGroupMemberNum）。
	MapGroupMemberNum(ctx context.Context, groupIDs []string) (map[string]int64, error)
	// ListMemberUserIDs 返回群全部成员 userID（无序）。
	ListMemberUserIDs(ctx context.Context, groupID string) ([]string, error)
	// MapGroupOwners 批量查询各群当前群主 userID（role=owner）。
	MapGroupOwners(ctx context.Context, groupIDs []string) (map[string]string, error)
	// MemberExists 判断用户是否已是群成员。
	MemberExists(ctx context.Context, groupID, userID string) (bool, error)
	// ListGroupsOfUser 分页查询用户所属群组列表（含总数）。
	ListGroupsOfUser(ctx context.Context, userID string, offset, limit int) (members []*types.GroupMember, total int64, err error)

	// ----- 已加入群 version -----
	// IncrJoinVersion 递增用户已加入群列表 version 并写 changelog。
	IncrJoinVersion(ctx context.Context, userID string, groupIDs []string, state int8) error
	// EnsureJoinGroupVersion 确保用户有 version 行。
	EnsureJoinGroupVersion(ctx context.Context, userID string) (*types.JoinGroupVersion, error)
	// GetJoinGroupVersion 读取水位。
	GetJoinGroupVersion(ctx context.Context, userID string) (*types.JoinGroupVersion, error)
	// ListJoinGroupVersionLogs 返回 (after, max] 变更日志。
	ListJoinGroupVersionLogs(ctx context.Context, userID string, afterVersion, maxVersion uint64) ([]*types.JoinGroupVersionLog, error)
	// ListJoinGroupIDs 返回用户已加入群 ID 有序列表。
	ListJoinGroupIDs(ctx context.Context, userID string) ([]string, error)

	// ----- 群成员 version -----
	// IncrMemberVersion 递增群成员列表 version 并写 changelog。
	IncrMemberVersion(ctx context.Context, groupID string, entityIDs []string, state int8) error
	// EnsureGroupMemberVersion 确保群有 version 行。
	EnsureGroupMemberVersion(ctx context.Context, groupID string) (*types.GroupMemberVersion, error)
	// GetGroupMemberVersion 读取水位。
	GetGroupMemberVersion(ctx context.Context, groupID string) (*types.GroupMemberVersion, error)
	// ListGroupMemberVersionLogs 返回 (after, max] 变更日志。
	ListGroupMemberVersionLogs(ctx context.Context, groupID string, afterVersion, maxVersion uint64) ([]*types.GroupMemberVersionLog, error)
	// ListMembersByIDs 按 userID 批量加载成员。
	ListMembersByIDs(ctx context.Context, groupID string, userIDs []string) ([]*types.GroupMember, error)
	// ListOrderedMemberUserIDs 返回有序成员 userID。
	ListOrderedMemberUserIDs(ctx context.Context, groupID string) ([]string, error)

	// ----- 入群请求 -----
	// CreateRequest 持久化入群请求。
	CreateRequest(ctx context.Context, r *types.GroupRequest) error
	// GetRequest 根据群组和用户 ID 查询入群请求，不存在则返回 ErrRequestNotFound。
	GetRequest(ctx context.Context, groupID, userID string) (*types.GroupRequest, error)
	// UpdateRequest 写入所有请求字段（包括零值）。
	UpdateRequest(ctx context.Context, r *types.GroupRequest) error
	// DeleteRequestsByGroup 删除群组所有入群请求。
	DeleteRequestsByGroup(ctx context.Context, groupID string) error
	// ListPendingByGroup 分页查询群组的待处理入群请求。
	ListPendingByGroup(ctx context.Context, groupID string, offset, limit int) (requests []*types.GroupRequest, total int64, err error)
	// ListByUser 分页查询用户的入群申请记录。
	ListByUser(ctx context.Context, userID string, offset, limit int) (requests []*types.GroupRequest, total int64, err error)
	// CountPendingByGroup 统计群组待处理的入群请求数量。
	CountPendingByGroup(ctx context.Context, groupID string) (int64, error)
}

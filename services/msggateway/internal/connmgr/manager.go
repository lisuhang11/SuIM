// Package connmgr 管理所有在线 WebSocket 连接，支持按用户/平台维度查询与操作。
package connmgr

import (
	"log/slog"
	"sync"

	"msggateway/internal/types"
)

// PlatformStatus 表示单一平台上token粒度管理。
type PlatformStatus struct {
	Tokens map[string]bool
	Total  int32
}

// Manager 线程安全的连接管理器。
// 索引：userID → platformID → connID → WritableConn
type Manager struct {
	mu sync.RWMutex

	// conns: userID -> platformID -> connID -> types.WritableConn
	conns map[string]map[int32]map[string]types.WritableConn
	// tokens: userID -> platformID -> token -> exists
	tokens map[string]map[int32]*PlatformStatus

	// 每个 connID 对应的 userID，用于 O(1) 反查。
	connUser map[string]string

	maxConnPerUser int
}

// New 创建连接管理器。
func New(maxConnPerUser int) *Manager {
	return &Manager{
		conns:          make(map[string]map[int32]map[string]types.WritableConn),
		tokens:         make(map[string]map[int32]*PlatformStatus),
		connUser:       make(map[string]string),
		maxConnPerUser: maxConnPerUser,
	}
}

// Add 注册新连接。若超过单用户限制则返回 false。
func (m *Manager) Add(userID string, platformID int32, token string, conn types.WritableConn) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.maxConnPerUser > 0 && m.countUserConnsLocked(userID) >= m.maxConnPerUser {
		return false
	}

	if m.conns[userID] == nil {
		m.conns[userID] = make(map[int32]map[string]types.WritableConn)
	}
	if m.conns[userID][platformID] == nil {
		m.conns[userID][platformID] = make(map[string]types.WritableConn)
	}
	m.conns[userID][platformID][conn.ID()] = conn

	if m.tokens[userID] == nil {
		m.tokens[userID] = make(map[int32]*PlatformStatus)
	}
	if m.tokens[userID][platformID] == nil {
		m.tokens[userID][platformID] = &PlatformStatus{Tokens: make(map[string]bool)}
	}
	m.tokens[userID][platformID].Tokens[token] = true
	m.tokens[userID][platformID].Total = int32(len(m.tokens[userID][platformID].Tokens))

	m.connUser[conn.ID()] = userID

	slog.Info("connection added",
		"user_id", userID,
		"platform_id", platformID,
		"conn_id", conn.ID(),
		"total_conns", m.totalConnsLocked(),
	)
	return true
}

// Remove 移除连接并清理空索引。
func (m *Manager) Remove(userID string, platformID int32, connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if userConns, ok := m.conns[userID]; ok {
		if platConns, ok2 := userConns[platformID]; ok2 {
			delete(platConns, connID)
			if len(platConns) == 0 {
				delete(userConns, platformID)
			}
		}
		if len(userConns) == 0 {
			delete(m.conns, userID)
		}
	}

	delete(m.connUser, connID)

	slog.Info("connection removed",
		"user_id", userID,
		"platform_id", platformID,
		"conn_id", connID,
		"total_conns", m.totalConnsLocked(),
	)
}

// RemoveToken 移除指定 token。
func (m *Manager) RemoveToken(userID string, platformID int32, token string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if userTokens, ok := m.tokens[userID]; ok {
		if ps, ok2 := userTokens[platformID]; ok2 {
			delete(ps.Tokens, token)
			ps.Total = int32(len(ps.Tokens))
			if len(ps.Tokens) == 0 {
				delete(userTokens, platformID)
			}
		}
		if len(userTokens) == 0 {
			delete(m.tokens, userID)
		}
	}
}

// GetUserConns 获取用户所有连接（全部平台）。
func (m *Manager) GetUserConns(userID string) []types.WritableConn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []types.WritableConn
	if userConns, ok := m.conns[userID]; ok {
		for _, platConns := range userConns {
			for _, conn := range platConns {
				result = append(result, conn)
			}
		}
	}
	return result
}

// GetUserPlatformConns 获取用户指定平台的所有连接。
func (m *Manager) GetUserPlatformConns(userID string, platformID int32) []types.WritableConn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []types.WritableConn
	if userConns, ok := m.conns[userID]; ok {
		if platConns, ok2 := userConns[platformID]; ok2 {
			for _, conn := range platConns {
				result = append(result, conn)
			}
		}
	}
	return result
}

// GetOnlineStatus 返回用户在线状态详情。status: 1=online, 0=offline。
func (m *Manager) GetOnlineStatus(userID string) (status int32, platforms []types.PlatformInfo) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userConns, ok := m.conns[userID]
	if !ok || len(userConns) == 0 {
		return 0, nil
	}

	status = 1
	userTokens := m.tokens[userID]
	for platID, platConns := range userConns {
		info := types.PlatformInfo{
			PlatformID: platID,
			ConnIDs:    make([]string, 0, len(platConns)),
		}
		for connID := range platConns {
			info.ConnIDs = append(info.ConnIDs, connID)
		}
		if userTokens != nil && userTokens[platID] != nil {
			for t := range userTokens[platID].Tokens {
				info.Tokens = append(info.Tokens, t)
			}
		}
		platforms = append(platforms, info)
	}
	return status, platforms
}

// KickUser 断开用户在指定平台上的所有连接。platformID=0 表示全部平台。
func (m *Manager) KickUser(userID string, platformID int32) int {
	m.mu.RLock()
	userConns, ok := m.conns[userID]
	if !ok {
		m.mu.RUnlock()
		return 0
	}
	var conns []types.WritableConn
	if platformID == 0 {
		for _, platConns := range userConns {
			for _, conn := range platConns {
				conns = append(conns, conn)
			}
		}
	} else {
		if platConns, ok2 := userConns[platformID]; ok2 {
			for _, conn := range platConns {
				conns = append(conns, conn)
			}
		}
	}
	m.mu.RUnlock()

	for _, conn := range conns {
		conn.Close(types.CloseCodeKicked, "kicked by server")
	}
	return len(conns)
}

// TotalConns 返回当前在线连接总数。
func (m *Manager) TotalConns() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalConnsLocked()
}

// OnlineUsers 返回当前在线用户数。
func (m *Manager) OnlineUsers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}

// ---------- 内部辅助 ----------

func (m *Manager) totalConnsLocked() int {
	total := 0
	for _, userConns := range m.conns {
		for _, platConns := range userConns {
			total += len(platConns)
		}
	}
	return total
}

func (m *Manager) countUserConnsLocked(userID string) int {
	total := 0
	if userConns, ok := m.conns[userID]; ok {
		for _, platConns := range userConns {
			total += len(platConns)
		}
	}
	return total
}

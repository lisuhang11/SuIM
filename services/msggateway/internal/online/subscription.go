package online

import (
	"sync"

	"msggateway/internal/types"
)

// Subscription 本机订阅表：被观察者 userID → 订阅者连接。
type Subscription struct {
	mu      sync.RWMutex
	targets map[string]map[string]types.WritableConn // targetUID → connID → conn
	byConn  map[string]map[string]struct{}           // connID → targetUID set
}

// NewSubscription 创建空订阅表。
func NewSubscription() *Subscription {
	return &Subscription{
		targets: make(map[string]map[string]types.WritableConn),
		byConn:  make(map[string]map[string]struct{}),
	}
}

// Sub 为 conn 增加/取消对若干 userID 的订阅。
func (s *Subscription) Sub(conn types.WritableConn, add, del []string) {
	if conn == nil {
		return
	}
	cid := conn.ID()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byConn[cid] == nil {
		s.byConn[cid] = make(map[string]struct{})
	}
	for _, uid := range del {
		if uid == "" {
			continue
		}
		delete(s.byConn[cid], uid)
		if m := s.targets[uid]; m != nil {
			delete(m, cid)
			if len(m) == 0 {
				delete(s.targets, uid)
			}
		}
	}
	for _, uid := range add {
		if uid == "" {
			continue
		}
		if _, ok := s.byConn[cid][uid]; ok {
			continue
		}
		s.byConn[cid][uid] = struct{}{}
		if s.targets[uid] == nil {
			s.targets[uid] = make(map[string]types.WritableConn)
		}
		s.targets[uid][cid] = conn
	}
	if len(s.byConn[cid]) == 0 {
		delete(s.byConn, cid)
	}
}

// DelConn 连接断开时清理其全部订阅。
func (s *Subscription) DelConn(conn types.WritableConn) {
	if conn == nil {
		return
	}
	cid := conn.ID()
	s.mu.Lock()
	defer s.mu.Unlock()
	targets := s.byConn[cid]
	delete(s.byConn, cid)
	for uid := range targets {
		if m := s.targets[uid]; m != nil {
			delete(m, cid)
			if len(m) == 0 {
				delete(s.targets, uid)
			}
		}
	}
}

// Subscribers 返回订阅了 targetUserID 的连接列表。
func (s *Subscription) Subscribers(targetUserID string) []types.WritableConn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.targets[targetUserID]
	if len(m) == 0 {
		return nil
	}
	out := make([]types.WritableConn, 0, len(m))
	for _, c := range m {
		out = append(out, c)
	}
	return out
}

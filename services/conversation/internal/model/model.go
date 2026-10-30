package model

import "time"

const (
	ConversationSingle       = 1
	ConversationGroup        = 2
	ConversationSuperGroup   = 3
	ConversationNotification = 4

	ReceiveMessage    = 0
	NotReceiveMessage = 1
	ReceiveNotNotify  = 2

	VersionInsert = 1
	VersionDelete = 2
	VersionUpdate = 3
)

type Conversation struct {
	OwnerUserID           string
	ConversationID        string
	ConversationType      int32
	UserID                string
	GroupID               string
	RecvMsgOpt            int32
	IsPinned              bool
	AttachedInfo          string
	IsPrivateChat         bool
	GroupAtType           int32
	Ex                    string
	BurnDuration          int32
	MinSeq                int64
	MaxSeq                int64
	MsgDestructTime       int64
	LatestMsgDestructTime time.Time
	IsMsgDestruct         bool
	CreateTime            time.Time
}

type Patch struct {
	RecvMsgOpt      *int32
	IsPinned        *bool
	AttachedInfo    *string
	Ex              *string
	GroupAtType     *int32
	MsgDestructTime *int64
	IsMsgDestruct   *bool
	BurnDuration    *int32
}

func (p Patch) Empty() bool {
	return p.RecvMsgOpt == nil && p.IsPinned == nil && p.AttachedInfo == nil && p.Ex == nil &&
		p.GroupAtType == nil && p.MsgDestructTime == nil && p.IsMsgDestruct == nil && p.BurnDuration == nil
}

type VersionHead struct {
	ID      string
	Version uint64
}

type VersionLog struct {
	ElemID  string
	State   int32
	Version uint64
}

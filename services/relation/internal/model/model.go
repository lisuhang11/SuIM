package model

import "time"

const (
	HandlePending int32 = 0
	HandleAgree   int32 = 1
	HandleRefuse  int32 = -1

	// AddSourceApply is the friend row created when a request is accepted.
	AddSourceApply int32 = 2

	// Version states match the version log: insert, delete, update.
	VersionInsert int32 = 1
	VersionDelete int32 = 2
	VersionUpdate int32 = 3

	// SortElemID is the extra friend version row written when pin state changes.
	SortElemID = "____S_O_R_T_I_D____"
)

type Friend struct {
	OwnerUserID    string
	FriendUserID   string
	Remark         string
	CreateTime     time.Time
	AddSource      int32
	OperatorUserID string
	IsPinned       bool
}

type Black struct {
	OwnerUserID    string
	BlockUserID    string
	CreateTime     time.Time
	AddSource      int32
	OperatorUserID string
}

type FriendRequest struct {
	FromUserID    string
	ToUserID      string
	HandleResult  int32
	ReqMsg        string
	CreateTime    time.Time
	HandlerUserID string
	HandleMsg     string
	HandleTime    time.Time
}

type UserProfile struct {
	UserID    string
	Nickname  string
	AvatarURL string
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

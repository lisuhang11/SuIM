package model

import "time"

const (
	GroupTypeNormal = 1
	GroupTypeSuper  = 2

	GroupStatusOK        = 0
	GroupStatusDismissed = 2

	RoleOwner    = 100
	RoleAdmin    = 60
	RoleOrdinary = 20

	JoinByInvitation = 2

	// NeedVerifyInviteDirect means an application needs approval and an invitation joins directly.
	NeedVerifyInviteDirect = 0
	// NeedVerifyAll means both applications and invitations from ordinary members need approval.
	NeedVerifyAll = 1

	VersionInsert = 1
	VersionDelete = 2
	VersionUpdate = 3

	// SortElemID is written when member roles change.
	SortElemID = "____S_O_R_T_I_D____"

	HandlePending = 0
)

type Group struct {
	GroupID                string
	GroupName              string
	Notification           string
	Introduction           string
	FaceURL                string
	CreateTime             time.Time
	Status                 int32
	CreatorUserID          string
	GroupType              int32
	NeedVerification       int32
	LookMemberInfo         int32
	ApplyMemberFriend      int32
	NotificationUpdateTime time.Time
	NotificationUserID     string
	Ex                     string
	MemberCount            int32
	OwnerUserID            string
}

type Member struct {
	GroupID        string
	UserID         string
	Nickname       string
	FaceURL        string
	RoleLevel      int32
	JoinTime       time.Time
	JoinSource     int32
	InviterUserID  string
	OperatorUserID string
	MuteEndTime    time.Time
	Ex             string
}

type Request struct {
	GroupID       string
	UserID        string
	HandleResult  int32
	ReqMsg        string
	JoinSource    int32
	InviterUserID string
	ReqTime       time.Time
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

type GroupPatch struct {
	Name               *string
	Notification       *string
	Introduction       *string
	FaceURL            *string
	NeedVerification   *int32
	LookMemberInfo     *int32
	ApplyMemberFriend  *int32
	Ex                 *string
	NotificationUserID string
	NotificationTime   time.Time
}

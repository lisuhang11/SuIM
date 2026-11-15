package model

import "time"

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
	// TokenTypeLegacy is the pre-redesign typ claim; treated as access when validating.
	TokenTypeLegacy = "token"
)

const (
	RevokeLogout   uint8 = 1
	RevokeKick     uint8 = 2
	RevokePassword uint8 = 3
	RevokeReplay   uint8 = 4
	RevokeBan      uint8 = 5
)

type User struct {
	UserID           string
	Email            string
	PasswordHash     string
	Nickname         string
	AvatarURL        string
	AppMangerLevel   int32
	GlobalRecvMsgOpt int32
	IsActive         bool
	AreaCode         string
	PhoneNumber      string
	Account          string
	Gender           int32
	BirthTime        time.Time
	Level            int32
	AllowVibration   int32
	AllowBeep        int32
	AllowAddFriend   int32
	RegisterType     int32
	Ex               string
	ProtocolVersion  string
	ProtocolAgreedAt time.Time
	RegisterIP       string
	RegisterReceipt  string
	CreateTime       time.Time
}

const RegisterTypePhone int32 = 1

// UserPatch 只包含本次要改的列。nil 表示保持原值。
type UserPatch struct {
	Nickname         *string
	AvatarURL        *string
	AppMangerLevel   *int32
	GlobalRecvMsgOpt *int32
	IsActive         *bool
	AreaCode         *string
	PhoneNumber      *string
	Account          *string
	Gender           *int32
	BirthTime        *time.Time
	ClearBirth       bool
	Level            *int32
	AllowVibration   *int32
	AllowBeep        *int32
	AllowAddFriend   *int32
	RegisterType     *int32
	Ex               *string
}

// Token is a session row. Raw is memory-only after Sign; JWT rows do not persist Raw or Hash.
type Token struct {
	ID           string
	UserID       string
	PlatformID   int32
	FamilyID     string
	ParentJTI    string
	Raw          string
	Hash         string
	Type         string
	ExpiresAt    time.Time
	RevokedAt    time.Time
	RevokeReason uint8
	IsRevoked    bool
}

// IsAccess reports whether the token type is usable as an API access credential.
func IsAccess(typ string) bool {
	return typ == TokenTypeAccess || typ == TokenTypeLegacy
}

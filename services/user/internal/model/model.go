package model

import "time"

const (
	TokenType = "token"
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
	CreateTime       time.Time
}

type Token struct {
	ID         string
	UserID     string
	PlatformID int32
	Raw        string
	Hash       string
	Type       string
	ExpiresAt  time.Time
	IsRevoked  bool
}

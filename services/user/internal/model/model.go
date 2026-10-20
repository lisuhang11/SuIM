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

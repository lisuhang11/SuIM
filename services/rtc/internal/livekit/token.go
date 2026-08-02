// Package livekit 提供 LiveKit 进房 JWT 签发。
package livekit

import (
	"time"

	lksdk "github.com/livekit/protocol/auth"
)

// NewJoinToken 签发允许加入指定房间的 LiveKit token。
func NewJoinToken(apiKey, apiSecret, roomName, identity string) (string, error) {
	at := lksdk.NewAccessToken(apiKey, apiSecret)
	grant := &lksdk.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetValidFor(time.Hour)
	return at.ToJWT()
}

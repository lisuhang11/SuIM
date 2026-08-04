package sdk_struct

// IMConfig is passed to InitSDK.
type IMConfig struct {
	ApiAddr string `json:"apiAddr"`
	DataDir string `json:"dataDir"`
}

// ServerUser matches SuIM proto UserInfo JSON (snake_case from gateway).
type ServerUser struct {
	UserID           string `json:"user_id"`
	Nickname         string `json:"nickname"`
	Email            string `json:"email"`
	AvatarURL        string `json:"avatar_url"`
	Ex               string `json:"ex"`
	AppMangerLevel   int32  `json:"app_manger_level"`
	GlobalRecvMsgOpt int32  `json:"global_recv_msg_opt"`
	IsActive         bool   `json:"is_active"`
	CreateTime       int64  `json:"create_time"`
	UpdatedAt        int64  `json:"updated_at"`
}

// LocalUser is the SDK local/cache model (OpenIM-style camelCase for App callbacks).
type LocalUser struct {
	UserID           string `json:"userID"`
	Nickname         string `json:"nickname"`
	Email            string `json:"email"`
	FaceURL          string `json:"faceURL"`
	Ex               string `json:"ex"`
	GlobalRecvMsgOpt int32  `json:"globalRecvMsgOpt"`
	CreateTime       int64  `json:"createTime"`
	UpdatedAt        int64  `json:"updatedAt"`
}

// PublicUser is returned by GetUsersInfo.
type PublicUser struct {
	UserID     string `json:"userID"`
	Nickname   string `json:"nickname"`
	FaceURL    string `json:"faceURL"`
	Ex         string `json:"ex"`
	CreateTime int64  `json:"createTime"`
}

// UserInfoUpdate is SetSelfInfo input (partial update).
type UserInfoUpdate struct {
	Nickname  *string `json:"nickname,omitempty"`
	FaceURL   *string `json:"faceURL,omitempty"`
	AvatarURL *string `json:"avatarURL,omitempty"` // alias of FaceURL for SuIM
}

package suim_sdk_callback

// Base is the async API callback (same contract as OpenIM).
type Base interface {
	OnError(errCode int32, errMsg string)
	OnSuccess(data string)
}

// OnUserListener receives user module push-style events.
type OnUserListener interface {
	OnSelfInfoUpdated(userInfo string)
	OnUserStatusChanged(userOnlineStatus string)
}

// OnConnListener receives connection lifecycle events (minimal for Init).
type OnConnListener interface {
	OnConnecting()
	OnConnectSuccess()
	OnConnectFailed(errCode int32, errMsg string)
	OnKickedOffline()
	OnUserTokenExpired()
}

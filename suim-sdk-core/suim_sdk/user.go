package suim_sdk

import "SuIM/suim-sdk-core/suim_sdk_callback"

// GetUsersInfo batch gets public user info.
func GetUsersInfo(callback suim_sdk_callback.Base, operationID string, userIDs string) {
	call(callback, operationID, IMUserContext.User().GetUsersInfo, userIDs)
}

// SetSelfInfo updates the current user's profile.
func SetSelfInfo(callback suim_sdk_callback.Base, operationID string, userInfo string) {
	call(callback, operationID, IMUserContext.User().SetSelfInfo, userInfo)
}

// GetSelfUserInfo obtains the current login user (with local cache).
func GetSelfUserInfo(callback suim_sdk_callback.Base, operationID string) {
	call(callback, operationID, IMUserContext.User().GetSelfUserInfo)
}

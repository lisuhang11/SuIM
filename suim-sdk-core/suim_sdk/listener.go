package suim_sdk

import "SuIM/suim-sdk-core/suim_sdk_callback"

// SetUserListener registers user event callbacks.
func SetUserListener(listener suim_sdk_callback.OnUserListener) {
	listenerCall(IMUserContext.SetUserListener, listener)
}

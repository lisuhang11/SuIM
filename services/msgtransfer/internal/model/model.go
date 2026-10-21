package model

const (
	SessionSingle       int32 = 1
	SessionWriteGroup   int32 = 2
	SessionReadGroup    int32 = 3
	SessionNotification int32 = 4

	ContentTyping         int32 = 113
	NotificationBegin     int32 = 1000
	NotificationEnd       int32 = 5000
	ContentHasReadReceipt int32 = 2200

	StatusSending int32 = 1
	StatusSuccess int32 = 2

	// A missing option is treated as enabled.
	OptHistory         = "history"
	OptOfflinePush     = "offlinePush"
	OptUnreadCount     = "unreadCount"
	OptNotNotification = "isNotNotification"
	OptSendMsg         = "isSendMsg"
)

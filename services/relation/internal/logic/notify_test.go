package logic

import "testing"

func TestFriendNoticeOptions(t *testing.T) {
	chat := friendNoticeOptions(true, true)
	if !chat[optSendMsg] || !chat[optUnreadCount] || chat[optHistory] || !chat[optOfflinePush] || chat[optNotNotification] {
		t.Fatalf("agree options = %+v", chat)
	}
	push := friendNoticeOptions(false, true)
	if push[optSendMsg] || push[optUnreadCount] || push[optHistory] || !push[optOfflinePush] || push[optNotNotification] {
		t.Fatalf("apply options = %+v", push)
	}
	quiet := friendNoticeOptions(false, false)
	if quiet[optSendMsg] || quiet[optUnreadCount] || quiet[optHistory] || quiet[optOfflinePush] || quiet[optNotNotification] {
		t.Fatalf("quiet options = %+v", quiet)
	}
	if friendOfflineTitle(contentFriendApply) == "" || friendOfflineTitle(contentFriendDeleted) != "" {
		t.Fatal("offline title")
	}
}

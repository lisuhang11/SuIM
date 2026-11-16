package logic

import (
	"testing"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestPushContentPrefersOfflineInfo(t *testing.T) {
	msg := &msgpb.MsgData{
		ContentType: 101,
		ClientMsgId: "c1",
		OfflinePushInfo: &msgpb.OfflinePushInfo{
			Title: "alice", Desc: "hello", Ex: "ext",
			IosPushSound: "chime.aiff", IosBadgeCount: true, SignalInfo: "s1",
		},
	}
	title, content, opts := pushContent(msg)
	if title != "alice" || content != "hello" {
		t.Fatalf("title/content = %q/%q", title, content)
	}
	if opts.Ex != "ext" || opts.IOSPushSound != "chime.aiff" || !opts.IOSBadgeCount {
		t.Fatalf("opts = %+v", opts)
	}
	if opts.Signal == nil || opts.Signal.ClientMsgID != "s1" {
		t.Fatalf("signal = %+v", opts.Signal)
	}
}

func TestPushContentFallsBackByType(t *testing.T) {
	title, content, opts := pushContent(&msgpb.MsgData{ContentType: 102, ClientMsgId: "c2"})
	if title != "[Image]" || content != "[Image]" {
		t.Fatalf("title/content = %q/%q", title, content)
	}
	if opts.Signal == nil || opts.Signal.ClientMsgID != "c2" {
		t.Fatalf("signal = %+v", opts.Signal)
	}
}

func TestDefaultTitleUnknownType(t *testing.T) {
	if got := defaultTitle(99999); got != "[New message]" {
		t.Fatalf("default title = %q", got)
	}
}

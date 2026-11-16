package logic

import (
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

// pushContent 构造通知标题、正文和厂商选项，缺字段时按消息类型兜底。
func pushContent(msg *msgpb.MsgData) (string, string, *options.Opts) {
	opts := &options.Opts{}
	if info := msg.GetOfflinePushInfo(); info != nil {
		opts.Ex = info.GetEx()
		opts.IOSPushSound = info.GetIosPushSound()
		opts.IOSBadgeCount = info.GetIosBadgeCount()
		if info.GetSignalInfo() != "" {
			opts.Signal = &options.Signal{ClientMsgID: info.GetSignalInfo()}
		}
	}
	if opts.Signal == nil && msg.GetClientMsgId() != "" {
		opts.Signal = &options.Signal{ClientMsgID: msg.GetClientMsgId()}
	}
	title, content := "", ""
	if info := msg.GetOfflinePushInfo(); info != nil {
		title, content = info.GetTitle(), info.GetDesc()
	}
	if title == "" {
		title = defaultTitle(msg.GetContentType())
	}
	if content == "" {
		content = title
	}
	return title, content, opts
}

func defaultTitle(contentType int32) string {
	switch contentType {
	case 101:
		return "[Message]"
	case 102:
		return "[Image]"
	case 103:
		return "[Voice]"
	case 104:
		return "[Video]"
	case 105:
		return "[File]"
	case 106:
		return "[Mention]"
	default:
		return "[New message]"
	}
}

package logic

import (
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

type msgOptions map[string]bool

func (o msgOptions) enabled(key string) bool {
	v, ok := o[key]
	if !ok || v {
		return true
	}
	return false
}

func (o msgOptions) history() bool { return o.enabled(model.OptHistory) }

func (o msgOptions) notNotification() bool { return o.enabled(model.OptNotNotification) }

func (o msgOptions) sendMsg() bool { return o.enabled(model.OptSendMsg) }

func (o msgOptions) offlinePush() bool { return o.enabled(model.OptOfflinePush) }

func (o msgOptions) unreadCount() bool { return o.enabled(model.OptUnreadCount) }

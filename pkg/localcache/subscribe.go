package localcache

import (
	"strings"
	"sync"
)

// Subscription 把一组 key 前缀登记到一个发布 topic。
// 删除缓存时用前缀判断这条 key 该发到哪些 topic。
type Subscription struct {
	Topic string
	Keys  []string
}

var (
	once      sync.Once
	subscribe map[string][]string
)

func InitLocalCache(subs []Subscription) {
	once.Do(func() {
		subscribe = make(map[string][]string)
		for _, sub := range subs {
			if sub.Topic == "" || len(sub.Keys) == 0 {
				continue
			}
			subscribe[sub.Topic] = append(subscribe[sub.Topic], sub.Keys...)
		}
	})
}

func GetPublishKeysByTopic(topics []string, keys []string) map[string][]string {
	keysByTopic := make(map[string][]string, len(topics))
	for _, topic := range topics {
		keysByTopic[topic] = []string{}
	}
	for _, key := range keys {
		for _, topic := range topics {
			prefixes, ok := subscribe[topic]
			if !ok {
				continue
			}
			for _, prefix := range prefixes {
				if strings.HasPrefix(key, prefix) {
					keysByTopic[topic] = append(keysByTopic[topic], key)
					break
				}
			}
		}
	}
	return keysByTopic
}

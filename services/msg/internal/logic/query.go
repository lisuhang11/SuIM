package logic

import (
	"strings"

	"github.com/lisuhang11/SuIM/pkg/msgconv"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const orderDesc = 1

// seqWindow keeps sequences inside [minSeq, maxSeq]. Order only decides which side is the end.
func seqWindow(order int32, minSeq, maxSeq int64, seqs []int64) (wanted []int64, isEnd bool, endSeq int64) {
	for _, seq := range seqs {
		if seq <= 0 {
			continue
		}
		if maxSeq > 0 && seq >= minSeq && seq <= maxSeq {
			wanted = append(wanted, seq)
			continue
		}
		if order == orderDesc && seq < minSeq {
			isEnd = true
			endSeq = minSeq
			continue
		}
		if order != orderDesc && seq > maxSeq {
			isEnd = true
			endSeq = maxSeq
		}
	}
	return wanted, isEnd, endSeq
}

func canRead(conversationID, userID string) bool {
	if userID == "" {
		return false
	}
	rest := ""
	switch {
	case strings.HasPrefix(conversationID, "si_"):
		rest = strings.TrimPrefix(conversationID, "si_")
	case strings.HasPrefix(conversationID, "sn_"):
		rest = strings.TrimPrefix(conversationID, "sn_")
	default:
		return true
	}
	parts := strings.SplitN(rest, "_", 2)
	return len(parts) == 2 && (parts[0] == userID || parts[1] == userID)
}

func rangeIsEnd(order int32, begin, end, maxSeq int64) bool {
	if order == orderDesc {
		return begin <= 1
	}
	if maxSeq <= 0 {
		return true
	}
	return maxSeq <= end
}

func putPull(out *msgpb.PushMessages, conversationID string, pull *msgpb.PullMsgs) {
	if msgconv.IsNotificationConversation(conversationID) {
		out.NotificationMsgs[conversationID] = pull
		return
	}
	out.Msgs[conversationID] = pull
}

func groupConversationID(group JoinedGroup) string {
	return msgconv.GroupConversationID(group.ID, group.Super)
}

package logic

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/services/third/internal/blob"
)

const formExpire = 10 * time.Minute

type formTicket struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Cause       string `json:"cause"`
	Key         string `json:"key"`
	UserID      string `json:"user_id"`
	Sig         string `json:"sig"`
}

func (s *Server) seal(ticket formTicket) (string, error) {
	if len(s.secret) == 0 {
		return "", errorcode.ErrInternal.WithMsg("form secret is empty")
	}
	ticket.Sig = s.formSig(ticket)
	raw, err := json.Marshal(ticket)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Server) open(id string) (formTicket, error) {
	var ticket formTicket
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return ticket, errorcode.ErrThirdInvalidArgs.WithMsg("invalid id")
	}
	if err := json.Unmarshal(raw, &ticket); err != nil {
		return ticket, errorcode.ErrThirdInvalidArgs.WithMsg("invalid id")
	}
	if len(s.secret) == 0 || !hmac.Equal([]byte(ticket.Sig), []byte(s.formSig(ticket))) {
		return ticket, errorcode.ErrThirdInvalidArgs.WithMsg("invalid id")
	}
	return ticket, nil
}

func (s *Server) formSig(ticket formTicket) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(strings.Join([]string{
		ticket.Name,
		ticket.ContentType,
		ticket.Cause,
		ticket.Key,
		ticket.UserID,
	}, "\n")))
	var size [8]byte
	n := ticket.Size
	for i := 7; i >= 0; i-- {
		size[i] = byte(n)
		n >>= 8
	}
	_, _ = mac.Write(size[:])
	return hex.EncodeToString(mac.Sum(nil))
}

func directKey(userID, name string, now time.Time) (string, error) {
	id, err := newUploadID()
	if err != nil {
		return "", err
	}
	return "direct/" + now.UTC().Format("20060102") + "/" + userID + "/" + id + fileExt(name), nil
}

func fileExt(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if len(ext) < 2 || len(ext) > 9 {
		return ""
	}
	for _, c := range ext[1:] {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return ""
		}
	}
	return ext
}

func validDirectKey(key string) bool {
	if !strings.HasPrefix(key, "direct/") || strings.Contains(key, "..") || strings.Contains(key, `\`) {
		return false
	}
	return true
}

func objectHash(etag string) string {
	etag = blob.NormalizeETag(etag)
	if validID(etag) {
		return etag
	}
	sum := md5.Sum([]byte(etag))
	return hex.EncodeToString(sum[:])
}

func int32s(values []int) []int32 {
	out := make([]int32, len(values))
	for i, v := range values {
		out[i] = int32(v)
	}
	return out
}

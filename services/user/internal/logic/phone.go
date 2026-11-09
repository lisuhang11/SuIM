package logic

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

const (
	SceneRegister = "register"
	SceneLogin    = "login"
	SceneBind     = "bind"
	SceneResetPwd = "resetpwd"
	phoneArea     = "86"
)

func validScene(scene string) bool {
	switch scene {
	case SceneRegister, SceneLogin, SceneBind, SceneResetPwd:
		return true
	default:
		return false
	}
}

func sceneNeedsAccount(scene string) bool {
	return scene == SceneLogin || scene == SceneResetPwd
}

// 接受可选的 +86、86 或 0086，忽略空格和横线。国内号码必须是 1 开头的 11 位。
func NormalizePhone(raw string) (area, national string, err error) {
	compact := compactPhone(raw)
	switch {
	case strings.HasPrefix(compact, "+"):
		if !strings.HasPrefix(compact, "+86") {
			return "", "", errorcode.ErrUserInvalidPhone
		}
		compact = compact[3:]
	case strings.HasPrefix(compact, "0086"):
		compact = compact[4:]
	case strings.HasPrefix(compact, phoneArea) && validNational(compact[len(phoneArea):]):
		compact = compact[len(phoneArea):]
	}
	if !validNational(compact) {
		return "", "", errorcode.ErrUserInvalidPhone
	}
	return phoneArea, compact, nil
}

func compactPhone(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range strings.TrimSpace(raw) {
		switch r {
		case ' ', '-', '\t':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func validNational(s string) bool {
	return len(s) == 11 && s[0] == '1' && digitsOnly(s)
}

func phoneKey(area, national string) string {
	return area + national
}

func providerPhone(area, national string) string {
	return "+" + area + national
}

func phoneNickname(raw, national string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = "用户" + national[len(national)-4:]
	}
	if utf8.RuneCountInString(name) > 255 {
		return "", errorcode.ErrUserInvalidUsername
	}
	return name, nil
}

func codeHash(pepper, scene, area, national, code string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(scene))
	mac.Write([]byte{0})
	mac.Write([]byte(area))
	mac.Write([]byte{0})
	mac.Write([]byte(national))
	mac.Write([]byte{0})
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomCode() (string, error) {
	for {
		var b [3]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		n := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1000000
		if n == 0 {
			continue
		}
		return fmt.Sprintf("%06d", n), nil
	}
}

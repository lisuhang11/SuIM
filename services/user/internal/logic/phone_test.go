package logic

import "testing"

func TestNormalizePhone(t *testing.T) {
	wantArea, wantNational := "86", "13800138000"
	for _, raw := range []string{
		"13800138000",
		" 138-0013-8000 ",
		"+86 138 0013 8000",
		"8613800138000",
		"008613800138000",
		"+8613800138000",
	} {
		area, national, err := NormalizePhone(raw)
		if err != nil || area != wantArea || national != wantNational {
			t.Fatalf("%q -> %s %s err=%v", raw, area, national, err)
		}
	}
	for _, raw := range []string{"", "+", "123", "23800138000", "+1 3800138000", "+13800138000", "138001380000", "86", "0086", "1380013800a"} {
		if _, _, err := NormalizePhone(raw); err == nil {
			t.Fatalf("%q accepted", raw)
		}
	}
}

func TestRandomCodeSkipsZero(t *testing.T) {
	for i := 0; i < 20; i++ {
		code, err := randomCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 || code == "000000" {
			t.Fatalf("code %q", code)
		}
	}
}

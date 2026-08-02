package repository

import (
	"testing"
	"time"
)

func TestWithinRevokeWindow(t *testing.T) {
	now := time.UnixMilli(1_700_000_120_000) // fixed

	cases := []struct {
		name     string
		sendTime int64
		want     bool
	}{
		{"fresh ms", now.Add(-30 * time.Second).UnixMilli(), true},
		{"at limit", now.Add(-2 * time.Minute).UnixMilli(), true},
		{"expired", now.Add(-2*time.Minute - time.Second).UnixMilli(), false},
		{"seconds unit", now.Add(-30 * time.Second).Unix(), true},
		{"zero", 0, false},
		{"future", now.Add(time.Minute).UnixMilli(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinRevokeWindow(tc.sendTime, now); got != tc.want {
				t.Fatalf("send=%d got=%v want=%v", tc.sendTime, got, tc.want)
			}
		})
	}
}

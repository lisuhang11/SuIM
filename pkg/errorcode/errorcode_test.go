package errorcode

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSentinelStatus(t *testing.T) {
	if status.Code(ErrUserEmailTaken) != codes.AlreadyExists {
		t.Fatalf("grpc code = %v", status.Code(ErrUserEmailTaken))
	}
	code, ok := CodeOf(ErrUserEmailTaken)
	if !ok || code != UserEmailTaken {
		t.Fatalf("code = %d ok=%v", code, ok)
	}
	got, ok := Lookup(UserEmailTaken)
	if !ok || got != ErrUserEmailTaken {
		t.Fatal("lookup missed sentinel")
	}
}

func TestWithMsgKeepsCode(t *testing.T) {
	err := ErrInternal.WithMsg("hash password")
	if err.Error() != "hash password" || err.Code() != Internal {
		t.Fatalf("copy = %+v", err)
	}
	if !errors.Is(err, ErrInternal) {
		t.Fatal("withmsg should match the sentinel")
	}
}

func TestCodeOfStatus(t *testing.T) {
	err := status.Convert(ErrUserInvalidToken).Err()
	code, ok := CodeOf(err)
	if !ok || code != UserInvalidToken {
		t.Fatalf("code = %d ok=%v", code, ok)
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("grpc code = %v", status.Code(err))
	}
}

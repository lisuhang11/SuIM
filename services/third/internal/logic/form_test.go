package logic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

func TestFormUpload(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 23, 1, 0, 0, 0, time.UTC)
	store := &memParts{}
	objects := &memObjects{}
	srv := New(&memSessions{}, objects, store, Options{
		Limits: DefaultLimits,
		Secret: "0123456789abcdef",
		Now:    func() time.Time { return now },
	})

	opened, err := srv.InitiateFormData(ctx, &thirdpb.InitiateFormDataReq{
		UserId: "u1", Name: "u1/pic.png", Size: 4, ContentType: "image/png", Cause: "msg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened.GetUrl() != "http://127.0.0.1:9000/suim" || opened.GetFile() != "file" {
		t.Fatalf("credential %+v", opened)
	}
	if opened.GetExpires() != now.Add(formExpire).UnixMilli() {
		t.Fatalf("expires %d", opened.GetExpires())
	}
	if len(opened.GetSuccessCodes()) != 1 || opened.GetSuccessCodes()[0] != 200 {
		t.Fatalf("codes %v", opened.GetSuccessCodes())
	}
	key := opened.GetFormData()["key"]
	if !strings.HasPrefix(key, "direct/20261023/u1/") || !strings.HasSuffix(key, ".png") {
		t.Fatalf("key %s", key)
	}

	if _, err := srv.CompleteFormData(ctx, &thirdpb.CompleteFormDataReq{
		UserId: "u1", Id: opened.GetId(), UrlPrefix: "https://cdn/",
	}); err == nil || !errors.Is(err, errorcode.ErrThirdNotFound) {
		t.Fatalf("missing object err=%v", err)
	}
	if _, err := objects.FindByName(ctx, "u1/pic.png"); err != model.ErrNotFound {
		t.Fatalf("row written early: %v", err)
	}

	store.setHead(key, 3, `"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	if _, err := srv.CompleteFormData(ctx, &thirdpb.CompleteFormDataReq{
		UserId: "u1", Id: opened.GetId(),
	}); err == nil || !errors.Is(err, errorcode.ErrThirdSizeMismatch) {
		t.Fatalf("size err=%v", err)
	}

	store.setHead(key, 4, `"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"`)
	done, err := srv.CompleteFormData(ctx, &thirdpb.CompleteFormDataReq{
		UserId: "u1", Id: opened.GetId(), UrlPrefix: "https://cdn/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.GetUrl() != "https://cdn/u1/pic.png" {
		t.Fatalf("url %s", done.GetUrl())
	}
	row, err := objects.FindByName(ctx, "u1/pic.png")
	if err != nil {
		t.Fatal(err)
	}
	if row.StorageKey != key || row.Size != 4 || row.Hash != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || row.UserID != "u1" {
		t.Fatalf("row %+v", row)
	}

	if _, err := srv.CompleteFormData(ctx, &thirdpb.CompleteFormDataReq{
		UserId: "u2", Id: opened.GetId(), UrlPrefix: "https://cdn/",
	}); err == nil || !errors.Is(err, errorcode.ErrNoPermission) {
		t.Fatalf("other user err=%v", err)
	}
	if _, err := srv.CompleteFormData(ctx, &thirdpb.CompleteFormDataReq{
		UserId: "u1", Id: opened.GetId() + "x",
	}); err == nil || !errors.Is(err, errorcode.ErrThirdInvalidArgs) {
		t.Fatalf("bad id err=%v", err)
	}
}

func (m *memParts) setHead(key string, size int64, etag string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.heads == nil {
		m.heads = map[string]memHead{}
	}
	m.heads[key] = memHead{size: size, etag: etag}
}

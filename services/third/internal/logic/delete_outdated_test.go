package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

func TestDeleteOutdatedDataRequiresExpireTime(t *testing.T) {
	srv := New(&memSessions{}, &memObjects{}, &memParts{}, Options{Limits: DefaultLimits, Secret: "x"})
	if _, err := srv.DeleteOutdatedData(context.Background(), &thirdpb.DeleteOutdatedDataReq{}); err == nil || !errors.Is(err, errorcode.ErrThirdInvalidArgs) {
		t.Fatalf("expire_time err=%v", err)
	}
}

// TestDeleteOutdatedDataKeepsSharedBytes: deleting one name that shares a storage
// key must not drop MinIO bytes while another name still points at it.
func TestDeleteOutdatedDataKeepsSharedBytes(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	objects := &memObjects{}
	parts := &memParts{objects: map[string][]byte{"objects/shared": []byte("payload")}}
	srv := New(&memSessions{}, objects, parts, Options{
		Limits: DefaultLimits,
		Secret: "x",
		Now:    func() time.Time { return now },
	})
	old := now.Add(-2 * time.Hour)
	fresh := now.Add(-time.Minute)
	if err := objects.InsertObject(ctx, &model.Object{
		Name: "u1/a.bin", UserID: "u1", Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		StorageKey: "objects/shared", Size: 7, Cause: "msg", CreateTime: old,
	}); err != nil {
		t.Fatal(err)
	}
	if err := objects.InsertObject(ctx, &model.Object{
		Name: "u1/b.bin", UserID: "u1", Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		StorageKey: "objects/shared", Size: 7, Cause: "msg", CreateTime: fresh,
	}); err != nil {
		t.Fatal(err)
	}
	cutAt := now.Add(-time.Hour)
	expired, err := objects.FindExpired(ctx, cutAt, nil, 10)
	if err != nil || len(expired) != 1 || expired[0].Name != "u1/a.bin" {
		t.Fatalf("fixture expired=%+v err=%v", expired, err)
	}

	resp, err := srv.DeleteOutdatedData(ctx, &thirdpb.DeleteOutdatedDataReq{ExpireTime: cutAt.UnixMilli(), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetCount() != 1 {
		t.Fatalf("count=%d", resp.GetCount())
	}
	if _, err := objects.FindByName(ctx, "u1/a.bin"); err != model.ErrNotFound {
		t.Fatalf("old name remains: %v", err)
	}
	if _, err := objects.FindByName(ctx, "u1/b.bin"); err != nil {
		t.Fatalf("fresh name removed: %v", err)
	}
	if _, ok := parts.objects["objects/shared"]; !ok {
		t.Fatal("shared bytes deleted while a name still references them")
	}

	resp, err = srv.DeleteOutdatedData(ctx, &thirdpb.DeleteOutdatedDataReq{ExpireTime: now.UnixMilli(), ObjectGroup: []string{"msg"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetCount() != 1 {
		t.Fatalf("second count=%d", resp.GetCount())
	}
	if _, ok := parts.objects["objects/shared"]; ok {
		t.Fatal("bytes left after last name was removed")
	}
}

func TestAuthSignRejectsExpiredAndForeign(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 23, 1, 0, 0, 0, time.UTC)
	sessions := &memSessions{}
	store := &memParts{}
	srv := New(sessions, &memObjects{}, store, Options{
		Limits: Limits{MinPartSize: 4, MaxPartSize: 16, MaxNumSize: 4},
		Expire: time.Hour,
		Now:    func() time.Time { return now },
	})
	opened, err := srv.InitiateMultipartUpload(ctx, &thirdpb.InitiateMultipartUploadReq{
		UserId: "u1", Hash: joinHash([]string{md5hex([]byte("abcd"))}), Size: 4,
		Name: "u1/x.bin", MaxParts: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	uploadID := opened.GetUpload().GetUploadId()
	if _, err := srv.AuthSign(ctx, &thirdpb.AuthSignReq{
		UserId: "u2", UploadId: uploadID, PartNumbers: []int32{1},
	}); err == nil || !errors.Is(err, errorcode.ErrNoPermission) {
		t.Fatalf("foreign user err=%v", err)
	}
	if _, err := srv.AuthSign(ctx, &thirdpb.AuthSignReq{
		UserId: "u1", UploadId: uploadID, PartNumbers: []int32{},
	}); err == nil || !errors.Is(err, errorcode.ErrThirdInvalidArgs) {
		t.Fatalf("empty parts err=%v", err)
	}
	srv.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := srv.AuthSign(ctx, &thirdpb.AuthSignReq{
		UserId: "u1", UploadId: uploadID, PartNumbers: []int32{1},
	}); err == nil || !errors.Is(err, errorcode.ErrThirdNotFound) {
		t.Fatalf("expired err=%v", err)
	}
}

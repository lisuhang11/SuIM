package logic

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/blob"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

func TestPlan(t *testing.T) {
	limits := Limits{MinPartSize: 4, MaxPartSize: 16, MaxNumSize: 4}
	size, count, err := Plan(limits, 16)
	if err != nil || size != 4 || count != 4 {
		t.Fatalf("small plan size=%d count=%d err=%v", size, count, err)
	}
	size, count, err = Plan(limits, 17)
	if err != nil || size != 5 || count != 4 {
		t.Fatalf("large plan size=%d count=%d err=%v", size, count, err)
	}
	if _, _, err := Plan(limits, 0); err == nil || !errors.Is(err, errorcode.ErrThirdInvalidArgs) {
		t.Fatalf("zero size err=%v", err)
	}
	if _, _, err := Plan(limits, 16*4+1); err == nil || !errors.Is(err, errorcode.ErrThirdTooLarge) {
		t.Fatalf("too large err=%v", err)
	}
}

func TestMultipartUpload(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 23, 1, 0, 0, 0, time.UTC)
	sessions := &memSessions{}
	objects := &memObjects{}
	store := &memParts{}
	srv := New(sessions, objects, store, Options{
		Limits: Limits{MinPartSize: 4, MaxPartSize: 16, MaxNumSize: 4},
		Expire: time.Hour,
		Now:    func() time.Time { return now },
	})
	limit, err := srv.PartLimit(ctx, &thirdpb.PartLimitReq{})
	if err != nil || limit.GetMinPartSize() != 4 || limit.GetMaxNumSize() != 4 {
		t.Fatalf("limit %+v err=%v", limit, err)
	}

	parts := [][]byte{[]byte("abcd"), []byte("efgh"), []byte("ij")}
	body := bytes.Join(parts, nil)
	sums := make([]string, len(parts))
	for i, p := range parts {
		sums[i] = md5hex(p)
	}
	hash := joinHash(sums)
	name := "u1/pic.bin"

	if _, err := srv.InitiateMultipartUpload(ctx, &thirdpb.InitiateMultipartUploadReq{
		UserId: "u1", Hash: hash, Size: int64(len(body)), Name: "pic.bin",
	}); err == nil || !errors.Is(err, errorcode.ErrNoPermission) {
		t.Fatalf("bare name err=%v", err)
	}

	opened, err := srv.InitiateMultipartUpload(ctx, &thirdpb.InitiateMultipartUploadReq{
		UserId: "u1", Hash: hash, Size: int64(len(body)), MaxParts: 1,
		Name: name, ContentType: "application/octet-stream", Cause: "msg", UrlPrefix: "https://cdn/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened.GetUpload().GetPartSize() != 4 || len(opened.GetUpload().GetSign().GetParts()) != 1 {
		t.Fatalf("opened %+v", opened.GetUpload())
	}
	if !strings.HasPrefix(opened.GetUpload().GetSign().GetParts()[0].GetUrl(), "http://127.0.0.1:9000/suim/") {
		t.Fatalf("sign url %s", opened.GetUpload().GetSign().GetParts()[0].GetUrl())
	}
	sess, err := sessions.GetSession(ctx, opened.GetUpload().GetUploadId())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.put(sess.StoreUploadID, 1, parts[0][:3]); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.CompleteMultipartUpload(ctx, &thirdpb.CompleteMultipartUploadReq{
		UserId: "u1", UploadId: opened.GetUpload().GetUploadId(), Parts: sums, Name: name,
	}); !errors.Is(err, errorcode.ErrThirdPartSize) {
		t.Fatalf("short part err=%v", err)
	}
	if err := store.put(sess.StoreUploadID, 1, parts[0]); err != nil {
		t.Fatal(err)
	}

	signed, err := srv.AuthSign(ctx, &thirdpb.AuthSignReq{
		UserId: "u1", UploadId: opened.GetUpload().GetUploadId(), PartNumbers: []int32{2, 3},
	})
	if err != nil || len(signed.GetParts()) != 2 {
		t.Fatalf("auth sign %+v err=%v", signed, err)
	}
	for i, part := range signed.GetParts() {
		if err := store.put(sess.StoreUploadID, int(part.GetPartNumber()), parts[i+1]); err != nil {
			t.Fatal(err)
		}
	}

	done, err := srv.CompleteMultipartUpload(ctx, &thirdpb.CompleteMultipartUploadReq{
		UserId: "u1", UploadId: opened.GetUpload().GetUploadId(), Parts: sums,
		Name: name, UrlPrefix: "https://cdn/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.GetUrl() != "https://cdn/"+name {
		t.Fatalf("url %s", done.GetUrl())
	}
	if !bytes.Equal(store.objects["objects/"+hash], body) {
		t.Fatalf("object bytes %q", store.objects["objects/"+hash])
	}
	if len(store.uploads) != 0 {
		t.Fatalf("multipart upload left behind: %d", len(store.uploads))
	}

	again, err := srv.CompleteMultipartUpload(ctx, &thirdpb.CompleteMultipartUploadReq{
		UserId: "u1", UploadId: opened.GetUpload().GetUploadId(), Parts: sums,
		Name: name, UrlPrefix: "https://cdn/",
	})
	if err != nil || again.GetUrl() != done.GetUrl() {
		t.Fatalf("repeat complete %+v err=%v", again, err)
	}

	instant, err := srv.InitiateMultipartUpload(ctx, &thirdpb.InitiateMultipartUploadReq{
		UserId: "u1", Hash: hash, Size: int64(len(body)), Name: "u1/copy.bin", UrlPrefix: "https://cdn/",
	})
	if err != nil || instant.GetUpload() != nil || instant.GetUrl() != "https://cdn/u1/copy.bin" {
		t.Fatalf("instant %+v err=%v", instant, err)
	}

	other := md5hex([]byte("zzzz"))
	otherHash := joinHash([]string{other})
	opened, err = srv.InitiateMultipartUpload(ctx, &thirdpb.InitiateMultipartUploadReq{
		UserId: "u1", Hash: otherHash, Size: 4, Name: name,
	})
	if err != nil {
		t.Fatal(err)
	}
	sess, err = sessions.GetSession(ctx, opened.GetUpload().GetUploadId())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.put(sess.StoreUploadID, 1, []byte("zzzz")); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.CompleteMultipartUpload(ctx, &thirdpb.CompleteMultipartUploadReq{
		UserId: "u1", UploadId: opened.GetUpload().GetUploadId(), Parts: []string{other}, Name: name,
	}); err == nil || !errors.Is(err, errorcode.ErrThirdNameConflict) {
		t.Fatalf("conflict err=%v", err)
	}
}

func md5hex(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

type memSessions struct {
	mu sync.Mutex
	m  map[string]*model.Session
}

func (m *memSessions) CreateSession(_ context.Context, sess *model.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = map[string]*model.Session{}
	}
	cp := *sess
	m.m[sess.ID] = &cp
	return nil
}

func (m *memSessions) GetSession(_ context.Context, id string) (*model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.m[id]
	if !ok {
		return nil, model.ErrUploadNotFound
	}
	cp := *sess
	return &cp, nil
}

func (m *memSessions) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, id)
	return nil
}

type memObjects struct {
	mu    sync.Mutex
	names map[string]*model.Object
}

func (m *memObjects) FindByHash(_ context.Context, hash string) (*model.Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, obj := range m.names {
		if obj.Hash == hash {
			cp := *obj
			return &cp, nil
		}
	}
	return nil, model.ErrNotFound
}

func (m *memObjects) FindByName(_ context.Context, name string) (*model.Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	obj, ok := m.names[name]
	if !ok {
		return nil, model.ErrNotFound
	}
	cp := *obj
	return &cp, nil
}

func (m *memObjects) InsertObject(_ context.Context, obj *model.Object) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.names == nil {
		m.names = map[string]*model.Object{}
	}
	if _, ok := m.names[obj.Name]; ok {
		return model.ErrNameTaken
	}
	cp := *obj
	m.names[obj.Name] = &cp
	return nil
}

type memUpload struct {
	parts map[int][]byte
}

type memParts struct {
	mu      sync.Mutex
	uploads map[string]*memUpload
	objects map[string][]byte
}

func (m *memParts) Create(_ context.Context, _ string, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.uploads == nil {
		m.uploads = map[string]*memUpload{}
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b[:])
	m.uploads[id] = &memUpload{parts: map[int][]byte{}}
	return id, nil
}

func (m *memParts) Sign(_ context.Context, key, uploadID string, part int, _ time.Duration) (string, error) {
	return fmt.Sprintf("http://127.0.0.1:9000/suim/%s?uploadId=%s&partNumber=%d", key, uploadID, part), nil
}

func (m *memParts) put(uploadID string, part int, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	up, ok := m.uploads[uploadID]
	if !ok {
		return model.ErrNotFound
	}
	up.parts[part] = append([]byte(nil), body...)
	return nil
}

func (m *memParts) List(_ context.Context, _, uploadID string) ([]model.StoredPart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	up, ok := m.uploads[uploadID]
	if !ok {
		return nil, model.ErrNotFound
	}
	out := make([]model.StoredPart, 0, len(up.parts))
	for n, body := range up.parts {
		out = append(out, model.StoredPart{
			Number: n,
			Size:   int64(len(body)),
			ETag:   `"` + md5hex(body) + `"`,
		})
	}
	return out, nil
}

func (m *memParts) Finish(_ context.Context, key, uploadID string, parts []model.StoredPart) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	up, ok := m.uploads[uploadID]
	if !ok {
		return model.ErrNotFound
	}
	if m.objects == nil {
		m.objects = map[string][]byte{}
	}
	var buf bytes.Buffer
	for _, part := range parts {
		body, ok := up.parts[part.Number]
		if !ok || blob.NormalizeETag(part.ETag) != md5hex(body) {
			return model.ErrNotFound
		}
		buf.Write(body)
	}
	m.objects[key] = buf.Bytes()
	delete(m.uploads, uploadID)
	return nil
}

func (m *memParts) Abort(_ context.Context, _, uploadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.uploads, uploadID)
	return nil
}

func (m *memParts) Stat(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	body, ok := m.objects[key]
	if !ok {
		return 0, model.ErrNotFound
	}
	return int64(len(body)), nil
}

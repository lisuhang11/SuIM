package model

import (
	"errors"
	"time"
)

var (
	// ErrNotFound means the object name or hash is not stored.
	ErrNotFound = errors.New("object not found")
	// ErrNameTaken means another object already uses this name.
	ErrNameTaken = errors.New("object name taken")
	// ErrUploadNotFound means the multipart session is missing or expired.
	ErrUploadNotFound = errors.New("upload not found")
)

// FormCredential is a browser POST policy for one object.
type FormCredential struct {
	URL          string
	File         string
	FormData     map[string]string
	Expires      time.Time
	SuccessCodes []int
}

// StoredPart is one part already accepted by object storage.
type StoredPart struct {
	Number int
	Size   int64
	ETag   string
}

// Object is one stored file name. Names that share a hash share StorageKey.
type Object struct {
	Name        string
	UserID      string
	Hash        string
	StorageKey  string
	Size        int64
	ContentType string
	Cause       string
	CreateTime  time.Time
}

// Session is an in-progress multipart upload.
// StoreUploadID is the MinIO upload id. ObjectKey is the MinIO object key.
type Session struct {
	ID            string
	UserID        string
	Hash          string
	Size          int64
	PartSize      int64
	PartCount     int
	ContentType   string
	Cause         string
	Name          string
	ExpireAt      time.Time
	StoreUploadID string
	ObjectKey     string
}

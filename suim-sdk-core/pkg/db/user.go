package db

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"SuIM/suim-sdk-core/pkg/sdkerrs"
	"SuIM/suim-sdk-core/sdk_struct"
)

// DataBase is a minimal local store for login user (JSON file), OpenIM-compatible method names.
type DataBase struct {
	mu       sync.RWMutex
	path     string
	loginUser *sdk_struct.LocalUser
}

func NewDataBase(dataDir, userID string) (*DataBase, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, "user_"+userID+".json")
	db := &DataBase{path: path}
	_ = db.load()
	return db, nil
}

func (d *DataBase) load() error {
	b, err := os.ReadFile(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var u sdk_struct.LocalUser
	if err := json.Unmarshal(b, &u); err != nil {
		return err
	}
	d.loginUser = &u
	return nil
}

func (d *DataBase) persist() error {
	if d.loginUser == nil {
		return nil
	}
	b, err := json.MarshalIndent(d.loginUser, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.path, b, 0o644)
}

func (d *DataBase) GetLoginUser(ctx context.Context, userID string) (*sdk_struct.LocalUser, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.loginUser == nil || d.loginUser.UserID != userID {
		return nil, sdkerrs.ErrRecordNotFound
	}
	cp := *d.loginUser
	return &cp, nil
}

func (d *DataBase) InsertLoginUser(ctx context.Context, user *sdk_struct.LocalUser) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := *user
	d.loginUser = &cp
	return d.persist()
}

func (d *DataBase) UpdateLoginUser(ctx context.Context, user *sdk_struct.LocalUser) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := *user
	d.loginUser = &cp
	return d.persist()
}

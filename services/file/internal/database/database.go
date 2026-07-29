package database

import (
	"fileservice/internal/config"
	"fileservice/internal/types"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func MustOpen(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("file database: %v", err))
	}
	if err := db.AutoMigrate(&types.File{}, &types.Binding{}, &types.AvatarBinding{}); err != nil {
		panic(fmt.Sprintf("file migrate: %v", err))
	}
	return db
}

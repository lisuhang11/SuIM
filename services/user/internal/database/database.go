// Package database initialises the GORM MySQL connection and runs auto-migration.
package database

import (
	"fmt"
	"log/slog"
	"user/internal/config"
	"user/internal/types"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// MustOpen opens a MySQL connection via GORM, runs auto-migration for the
// domain models, and returns the *gorm.DB. It panics on unrecoverable errors
// so the caller can defer the decision to crash or recover.
func MustOpen(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		panic(fmt.Sprintf("failed to connect to MySQL: %v", err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Sprintf("failed to get underlying sql.DB: %v", err))
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	if err := db.AutoMigrate(&types.User{}, &types.AuthToken{}); err != nil {
		panic(fmt.Sprintf("failed to auto-migrate: %v", err))
	}

	slog.Info("database connected and migrated")
	return db
}

// Package database 初始化 GORM MySQL 连接并执行自动迁移。
package database

import (
	"context"
	"fmt"

	"push/internal/config"
	"push/internal/logger"
	"push/internal/types"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// MustOpen 打开 MySQL 连接，自动迁移领域模型，返回 *gorm.DB。失败时 panic。
func MustOpen(ctx context.Context, cfg *config.Config) *gorm.DB {
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

	if err := db.AutoMigrate(&types.PushToken{}); err != nil {
		panic(fmt.Sprintf("failed to auto-migrate: %v", err))
	}

	logger.Info(ctx, "database connected and migrated")
	return db
}

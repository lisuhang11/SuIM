// Package database 初始化 GORM MySQL 连接并执行自动迁移。
package database

import (
	"fmt"
	"log/slog"

	"group/internal/config"
	"group/internal/types"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// MustOpen 打开 MySQL 连接，执行自动迁移，返回 *gorm.DB。
// 若发生不可恢复的错误则 panic。
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

	// 自动迁移群组、群成员和入群请求表结构。
	if err := db.AutoMigrate(&types.Group{}, &types.GroupMember{}, &types.GroupRequest{}); err != nil {
		panic(fmt.Sprintf("failed to auto-migrate: %v", err))
	}

	slog.Info("database connected and migrated")
	return db
}

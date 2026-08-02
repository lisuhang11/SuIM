// Package database 初始化 GORM MySQL 连接并执行自动迁移。
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

// MustOpen 打开 MySQL 连接，执行自动迁移，返回 *gorm.DB。
// 若发生不可恢复的错误则 panic，由调用方决定是否恢复。
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

	// 自动迁移用户和令牌表结构。
	if err := db.AutoMigrate(&types.User{}, &types.AuthToken{}); err != nil {
		panic(fmt.Sprintf("failed to auto-migrate: %v", err))
	}

	// 历史误用 GORM 默认复数表名 users，一次性并入正式表 user。
	if err := migrateLegacyUsersTable(db); err != nil {
		panic(fmt.Sprintf("failed to migrate legacy users table: %v", err))
	}

	slog.Info("database connected and migrated")
	return db
}

// migrateLegacyUsersTable 将历史 `users` 表数据并入 `user`，然后删除 `users`。
// 无 `users` 表时直接跳过。
func migrateLegacyUsersTable(db *gorm.DB) error {
	if !db.Migrator().HasTable("users") {
		return nil
	}

	slog.Info("migrating legacy users table into user")
	const copySQL = "" +
		"INSERT INTO `user` (" +
		"user_id, email, password_hash, nickname, avatar_url, ex, " +
		"app_manger_level, global_recv_msg_opt, is_active, create_time, updated_at" +
		") SELECT " +
		"user_id, email, password_hash, nickname, avatar_url, ex, " +
		"app_manger_level, global_recv_msg_opt, is_active, create_time, updated_at " +
		"FROM users " +
		"WHERE user_id NOT IN (SELECT user_id FROM `user`)"
	if err := db.Exec(copySQL).Error; err != nil {
		return fmt.Errorf("copy users -> user: %w", err)
	}

	if err := db.Migrator().DropTable("users"); err != nil {
		return fmt.Errorf("drop users: %w", err)
	}
	slog.Info("legacy users table migrated and dropped")
	return nil
}

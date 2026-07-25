-- SuIM user service database migration (MySQL 8.0+)

CREATE TABLE IF NOT EXISTS `user` (
    `user_id`             VARCHAR(64)   NOT NULL                COMMENT '用户ID',
    `email`               VARCHAR(255)  NOT NULL                COMMENT '邮箱',
    `password_hash`       VARCHAR(255)  NOT NULL                COMMENT '密码哈希',
    `nickname`            VARCHAR(255)  NOT NULL DEFAULT ''     COMMENT '昵称',
    `avatar_url`          VARCHAR(1024) NOT NULL DEFAULT ''     COMMENT '头像URL',
    `ex`                  VARCHAR(1024) NOT NULL DEFAULT ''     COMMENT '扩展字段',
    `app_manger_level`    INT           NOT NULL DEFAULT 0      COMMENT '管理员级别',
    `global_recv_msg_opt` INT           NOT NULL DEFAULT 0      COMMENT '全局消息接收选项',
    `is_active`           TINYINT(1)    NOT NULL DEFAULT 1      COMMENT '是否激活',
    `create_time`         DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    `updated_at`          DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (`user_id`),
    UNIQUE INDEX `idx_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户基础信息表';

CREATE TABLE IF NOT EXISTS `auth_tokens` (
    `id`          VARCHAR(36)  NOT NULL COMMENT '令牌ID',
    `user_id`     VARCHAR(64)  NOT NULL COMMENT '用户ID',
    `token`       TEXT         NOT NULL COMMENT 'JWT令牌原文',
    `token_type`  VARCHAR(50)  NOT NULL COMMENT 'access_token 或 refresh_token',
    `expires_at`  DATETIME(3)  NOT NULL COMMENT '过期时间',
    `is_revoked`  TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否已撤销',
    `created_at`  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    `updated_at`  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (`id`),
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_token` (`token`(128))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='认证令牌表';

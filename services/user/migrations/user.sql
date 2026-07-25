CREATE TABLE IF NOT EXISTS auth_tokens (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(36) NOT NULL,
    token TEXT NOT NULL, -- JWT 令牌原文
    token_type VARCHAR(50) NOT NULL,-- access 或 refresh
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT false,--是否已撤销
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE `user` (
    `user_id`             VARCHAR(64)   NOT NULL                COMMENT '用户ID',
    `nickname`            VARCHAR(255)  NOT NULL DEFAULT ''     COMMENT '昵称',
    `avatar_url`            VARCHAR(1024) NOT NULL DEFAULT ''     COMMENT '头像URL',
    `ex`                  VARCHAR(1024) NOT NULL DEFAULT ''     COMMENT '扩展字段',
    `app_manger_level`    INT           NOT NULL DEFAULT 0      COMMENT '管理员级别',
    `global_recv_msg_opt` INT           NOT NULL DEFAULT 0      COMMENT '全局消息接收选项',
    `create_time`         DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户基础信息表';


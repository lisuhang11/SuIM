-- push_token 存储用户的设备推送令牌，按 user_id + platform_id 唯一。
-- platform_id: 1=iOS, 2=Android, 3=Windows, 4=macOS, 5=Web。
CREATE TABLE IF NOT EXISTS push_token (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id     VARCHAR(64)  NOT NULL COMMENT '用户ID',
    platform_id INT          NOT NULL DEFAULT 0 COMMENT '平台ID',
    token       VARCHAR(512) NOT NULL COMMENT '设备推送令牌',
    created_at  BIGINT       NOT NULL DEFAULT 0 COMMENT '创建时间(epoch millis)',
    updated_at  BIGINT       NOT NULL DEFAULT 0 COMMENT '更新时间(epoch millis)',
    UNIQUE KEY idx_user_platform (user_id, platform_id),
    KEY         idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户设备推送令牌表';

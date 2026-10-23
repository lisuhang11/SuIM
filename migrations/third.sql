CREATE TABLE IF NOT EXISTS `object` (
    `name`         VARCHAR(512)  NOT NULL                COMMENT '对象名，必须以用户ID为第一段',
    `user_id`      VARCHAR(64)   NOT NULL                COMMENT '上传者',
    `hash`         CHAR(32)      NOT NULL                COMMENT '内容哈希',
    `storage_key`  VARCHAR(255)  NOT NULL                COMMENT 'MinIO对象键',
    `size`         BIGINT        NOT NULL                COMMENT '字节数',
    `content_type` VARCHAR(255)  NOT NULL DEFAULT ''     COMMENT '内容类型',
    `cause`        VARCHAR(64)   NOT NULL DEFAULT ''     COMMENT '业务来源',
    `create_time`  DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (`name`),
    KEY `idx_object_hash` (`hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='已完成的上传对象';

CREATE TABLE IF NOT EXISTS `friend` (
    `owner_user_id`    VARCHAR(64)  NOT NULL COMMENT '关系拥有者 userID',
    `friend_user_id`   VARCHAR(64)  NOT NULL COMMENT '好友 userID',
    `remark`           VARCHAR(255) DEFAULT '' COMMENT '好友备注',
    `create_time`      DATETIME     DEFAULT NULL COMMENT '成为好友的时间',
    `add_source`       INT          DEFAULT 0 COMMENT '添加来源',
    `operator_user_id` VARCHAR(64)  DEFAULT '' COMMENT '操作者 userID',
    `is_pinned`        TINYINT(1)   DEFAULT 0 COMMENT '是否置顶(1是0否)',
    PRIMARY KEY (`owner_user_id`, `friend_user_id`),
    INDEX `idx_friend_owner_pinned` (`owner_user_id`, `is_pinned` DESC, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友关系表(单方视角)';

CREATE TABLE IF NOT EXISTS `friend_request` (
    `from_user_id`    VARCHAR(64)  NOT NULL COMMENT '申请人 userID',
    `to_user_id`      VARCHAR(64)  NOT NULL COMMENT '被申请人 userID',
    `handle_result`   INT          DEFAULT 0 COMMENT '0未处理 1同意 -1拒绝',
    `req_msg`         VARCHAR(512) DEFAULT '' COMMENT '申请留言',
    `create_time`     DATETIME     DEFAULT NULL COMMENT '申请时间',
    `handler_user_id` VARCHAR(64)  DEFAULT '' COMMENT '处理人 userID',
    `handle_msg`      VARCHAR(512) DEFAULT '' COMMENT '处理留言',
    `handle_time`     DATETIME     DEFAULT NULL COMMENT '处理时间',
    PRIMARY KEY (`from_user_id`, `to_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友申请表';

CREATE TABLE IF NOT EXISTS `black` (
    `owner_user_id`    VARCHAR(64) NOT NULL COMMENT '拉黑者 userID',
    `block_user_id`    VARCHAR(64) NOT NULL COMMENT '被拉黑者 userID',
    `create_time`      DATETIME    DEFAULT NULL COMMENT '拉黑时间',
    `add_source`       INT         DEFAULT 0 COMMENT '添加来源',
    `operator_user_id` VARCHAR(64) DEFAULT '' COMMENT '操作者 userID',
    PRIMARY KEY (`owner_user_id`, `block_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='黑名单表(单方视角)';

-- 版本头。同一用户一行，id 在首次写入后不变。deleted 固定为 0。
-- 变更明细同一 owner + elem_id 只保留最新一条。
-- state: 1 insert, 2 delete, 3 update
-- 好友置顶变更额外写一行 elem_id = '____S_O_R_T_I_D____'
CREATE TABLE IF NOT EXISTS `friend_version` (
    `id`            CHAR(24)        NOT NULL COMMENT '版本ID，首次写入后不变',
    `owner_user_id` VARCHAR(64)     NOT NULL COMMENT '关系拥有者',
    `version`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前版本号',
    `deleted`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '固定为0',
    `last_update`   DATETIME(3)     NOT NULL COMMENT '最后更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_friend_version_owner` (`owner_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友版本';

CREATE TABLE IF NOT EXISTS `friend_version_log` (
    `owner_user_id` VARCHAR(64)     NOT NULL COMMENT '关系拥有者',
    `elem_id`       VARCHAR(64)     NOT NULL COMMENT '好友用户ID，置顶变更为排序哨兵',
    `state`         TINYINT         NOT NULL COMMENT '1插入 2删除 3更新',
    `version`       BIGINT UNSIGNED NOT NULL COMMENT '该元素的版本号',
    `last_update`   DATETIME(3)     NOT NULL COMMENT '最后更新时间',
    PRIMARY KEY (`owner_user_id`, `elem_id`),
    KEY `idx_friend_version_owner` (`owner_user_id`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友版本明细';

-- Blacks are a full list, same as OpenIM. Drop the unused version tables.
DROP TABLE IF EXISTS `black_version_log`;
DROP TABLE IF EXISTS `black_version`;

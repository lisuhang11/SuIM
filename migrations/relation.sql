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

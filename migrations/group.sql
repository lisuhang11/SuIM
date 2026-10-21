CREATE TABLE IF NOT EXISTS `group` (
    `group_id`                 VARCHAR(64)   NOT NULL COMMENT '群ID',
    `group_name`               VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '群名称',
    `notification`             TEXT          NULL COMMENT '群公告',
    `introduction`             VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群简介',
    `face_url`                 VARCHAR(512)  NOT NULL DEFAULT '' COMMENT '群头像',
    `create_time`              DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    `status`                   INT           NOT NULL DEFAULT 0 COMMENT '0正常 2已解散',
    `creator_user_id`          VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '创建者用户ID',
    `group_type`               INT           NOT NULL DEFAULT 1 COMMENT '群类型，1为普通群，2为超级群',
    `need_verification`        INT           NOT NULL DEFAULT 0 COMMENT '0邀请直接加入 1都需验证 2直接加入',
    `look_member_info`         INT           NOT NULL DEFAULT 0 COMMENT '0禁止查看成员 1允许',
    `apply_member_friend`      INT           NOT NULL DEFAULT 0 COMMENT '0禁止成员互加好友 1允许',
    `notification_update_time` DATETIME(3)   NULL COMMENT '公告更新时间',
    `notification_user_id`     VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '公告更新者',
    `ex`                       VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段',
    PRIMARY KEY (`group_id`),
    KEY `idx_group_name_create_time` (`group_name`, `create_time`),
    KEY `idx_group_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群资料';

CREATE TABLE IF NOT EXISTS `group_member` (
    `group_id`         VARCHAR(64)   NOT NULL COMMENT '群ID',
    `user_id`          VARCHAR(64)   NOT NULL COMMENT '成员用户ID',
    `nickname`         VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '群内昵称',
    `face_url`         VARCHAR(512)  NOT NULL DEFAULT '' COMMENT '群内头像',
    `role_level`       INT           NOT NULL DEFAULT 20 COMMENT '100群主 60管理员 20普通成员',
    `join_time`        DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '入群时间',
    `join_source`      INT           NOT NULL DEFAULT 2 COMMENT '入群来源，2为邀请',
    `inviter_user_id`  VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '邀请人',
    `operator_user_id` VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '操作者',
    `mute_end_time`    DATETIME(3)   NULL COMMENT '禁言截止时间，空表示未禁言',
    `ex`               VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段',
    PRIMARY KEY (`group_id`, `user_id`),
    KEY `idx_group_member_user_id` (`user_id`),
    KEY `idx_group_member_role` (`group_id`, `role_level`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群成员';

CREATE TABLE IF NOT EXISTS `group_request` (
    `group_id`        VARCHAR(64)   NOT NULL COMMENT '群ID',
    `user_id`         VARCHAR(64)   NOT NULL COMMENT '申请人',
    `handle_result`   INT           NOT NULL DEFAULT 0 COMMENT '0未处理 1同意 -1拒绝',
    `req_msg`         VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '申请留言',
    `handled_msg`     VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '处理留言',
    `req_time`        DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '申请时间',
    `handle_user_id`  VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '处理人',
    `handled_time`    DATETIME(3)   NULL COMMENT '处理时间',
    `join_source`     INT           NOT NULL DEFAULT 2 COMMENT '入群来源，2为邀请',
    `inviter_user_id` VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '邀请人',
    `ex`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段',
    PRIMARY KEY (`group_id`, `user_id`),
    KEY `idx_group_request_user_id` (`user_id`),
    KEY `idx_group_request_req_time` (`req_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='入群申请';

-- 每个群一行。version_id 返回给客户端，文档重建后必须换新值。
-- 明细里每个 e_id 只留最新一条。e_id 为 user_id、空串（群资料）或排序哨兵。
CREATE TABLE IF NOT EXISTS `group_member_version` (
    `version_id`  CHAR(24)        NOT NULL COMMENT '版本ID，文档重建后更换',
    `d_id`        VARCHAR(64)     NOT NULL COMMENT '群ID',
    `version`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前版本号',
    `deleted`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '固定为0',
    `last_update` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '最后更新时间',
    PRIMARY KEY (`d_id`),
    UNIQUE KEY `uk_group_member_version_id` (`version_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群成员版本';

CREATE TABLE IF NOT EXISTS `group_member_version_log` (
    `d_id`        VARCHAR(64)     NOT NULL COMMENT '群ID',
    `e_id`        VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '成员用户ID，空串表示群资料，或排序哨兵',
    `state`       INT             NOT NULL COMMENT '1插入 2删除 3更新',
    `version`     BIGINT UNSIGNED NOT NULL COMMENT '该元素的版本号',
    `last_update` DATETIME(3)     NOT NULL COMMENT '最后更新时间',
    PRIMARY KEY (`d_id`, `e_id`),
    KEY `idx_group_member_version_log_version` (`d_id`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群成员版本明细';

-- d_id = user_id，e_id = group_id
CREATE TABLE IF NOT EXISTS `group_join_version` (
    `version_id`  CHAR(24)        NOT NULL COMMENT '版本ID，文档重建后更换',
    `d_id`        VARCHAR(64)     NOT NULL COMMENT '用户ID',
    `version`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前版本号',
    `deleted`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '固定为0',
    `last_update` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '最后更新时间',
    PRIMARY KEY (`d_id`),
    UNIQUE KEY `uk_group_join_version_id` (`version_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户入群版本';

CREATE TABLE IF NOT EXISTS `group_join_version_log` (
    `d_id`        VARCHAR(64)     NOT NULL COMMENT '用户ID',
    `e_id`        VARCHAR(64)     NOT NULL COMMENT '群ID',
    `state`       INT             NOT NULL COMMENT '1插入 2删除 3更新',
    `version`     BIGINT UNSIGNED NOT NULL COMMENT '该元素的版本号',
    `last_update` DATETIME(3)     NOT NULL COMMENT '最后更新时间',
    PRIMARY KEY (`d_id`, `e_id`),
    KEY `idx_group_join_version_log_version` (`d_id`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户入群版本明细';

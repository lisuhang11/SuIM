-- 群组表
CREATE TABLE `group` (
                         `group_id`                VARCHAR(64)   NOT NULL COMMENT '群组ID',
                         `group_name`              VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '群名称',
                         `notification`            VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群公告',
                         `introduction`            VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群简介',
                         `face_url`                VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群头像',
                         `create_time`             DATETIME      NOT NULL COMMENT '创建时间',
                         `ex`                      VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段(json)',
                         `status`                  INT           NOT NULL DEFAULT 0 COMMENT '群状态',
                         `creator_user_id`         VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '创建者用户ID',
                         `group_type`              INT           NOT NULL DEFAULT 0 COMMENT '群类型',
                         `need_verification`       INT           NOT NULL DEFAULT 0 COMMENT '加群是否需要验证',
                         `look_member_info`        INT           NOT NULL DEFAULT 0 COMMENT '是否允许查看成员信息',
                         `apply_member_friend`     INT           NOT NULL DEFAULT 0 COMMENT '是否允许成员互加好友',
                         `notification_update_time` DATETIME     NOT NULL COMMENT '公告更新时间',
                         `notification_user_id`    VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '公告更新者用户ID',
                         PRIMARY KEY (`group_id`),
                         UNIQUE KEY `uk_group_id` (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群组信息表';

-- 群成员表
CREATE TABLE `group_member` (
                                `group_id`        VARCHAR(64)   NOT NULL COMMENT '群组ID',
                                `user_id`         VARCHAR(64)   NOT NULL COMMENT '用户ID',
                                `nickname`        VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '群内昵称',
                                `face_url`        VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群内头像',
                                `role_level`      INT           NOT NULL DEFAULT 0 COMMENT '角色(群主/管理员/普通成员)',
                                `join_time`       DATETIME      NOT NULL COMMENT '加入时间',
                                `join_source`     INT           NOT NULL DEFAULT 0 COMMENT '加入来源',
                                `inviter_user_id` VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '邀请者用户ID',
                                `operator_user_id` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '操作者用户ID',
                                `mute_end_time`   DATETIME      NOT NULL COMMENT '禁言结束时间',
                                `ex`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段(json)',
                                PRIMARY KEY (`group_id`, `user_id`),
                                UNIQUE KEY `uk_group_user` (`group_id`, `user_id`),
                                KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群成员表';

-- 加群申请表
CREATE TABLE `group_request` (
                                 `user_id`         VARCHAR(64)   NOT NULL COMMENT '申请用户ID',
                                 `group_id`        VARCHAR(64)   NOT NULL COMMENT '群组ID',
                                 `handle_result`   INT           NOT NULL DEFAULT 0 COMMENT '处理状态(0未处理/已同意/已拒绝)',
                                 `req_msg`         VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '申请留言',
                                 `handled_msg`     VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '处理留言',
                                 `req_time`        DATETIME      NOT NULL COMMENT '申请时间',
                                 `handle_user_id`  VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '处理者用户ID',
                                 `handled_time`    DATETIME      NOT NULL COMMENT '处理时间',
                                 `join_source`     INT           NOT NULL DEFAULT 0 COMMENT '加入来源',
                                 `inviter_user_id` VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '邀请者用户ID',
                                 `ex`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段(json)',
                                 PRIMARY KEY (`group_id`, `user_id`),
                                 UNIQUE KEY `uk_group_user` (`group_id`, `user_id`),
                                 KEY `idx_user_id` (`user_id`),
                                 KEY `idx_req_time` (`req_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='加群申请表';

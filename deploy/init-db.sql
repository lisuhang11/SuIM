-- ============================================================
-- SuIM database initialization
-- ============================================================
CREATE DATABASE IF NOT EXISTS `suim` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `suim`;

-- ============================================================
-- user service
-- ============================================================
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

-- ============================================================
-- relation service
-- ============================================================
CREATE TABLE IF NOT EXISTS `friend` (
    `owner_user_id`    VARCHAR(64)   NOT NULL COMMENT '关系拥有者 userID',
    `friend_user_id`   VARCHAR(64)   NOT NULL COMMENT '好友 userID',
    `remark`           VARCHAR(255)  DEFAULT '' COMMENT '好友备注',
    `create_time`      DATETIME      DEFAULT NULL COMMENT '成为好友的时间',
    `add_source`       INT           DEFAULT 0 COMMENT '添加来源',
    `operator_user_id` VARCHAR(64)   DEFAULT '' COMMENT '操作者 userID',
    `ex`               VARCHAR(1024) DEFAULT '' COMMENT '扩展字段(json)',
    `is_pinned`        TINYINT(1)    DEFAULT 0 COMMENT '是否置顶',
    PRIMARY KEY (`owner_user_id`, `friend_user_id`),
    INDEX `idx_friend_owner_pinned` (`owner_user_id`, `is_pinned` DESC, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友关系表';

CREATE TABLE IF NOT EXISTS `friend_request` (
    `from_user_id`    VARCHAR(64)  NOT NULL COMMENT '申请人 userID',
    `to_user_id`      VARCHAR(64)  NOT NULL COMMENT '被申请人 userID',
    `handle_result`   INT          DEFAULT 0 COMMENT '0未处理 1同意 -1拒绝',
    `req_msg`         VARCHAR(512) DEFAULT '' COMMENT '申请留言',
    `create_time`     DATETIME     DEFAULT NULL COMMENT '申请时间',
    `handler_user_id` VARCHAR(64)  DEFAULT '' COMMENT '处理人 userID',
    `handle_msg`      VARCHAR(512) DEFAULT '' COMMENT '处理留言',
    `handle_time`     DATETIME     DEFAULT NULL COMMENT '处理时间',
    `ex`              VARCHAR(1024) DEFAULT '' COMMENT '扩展字段(json)',
    PRIMARY KEY (`from_user_id`, `to_user_id`),
    INDEX `idx_friend_request_ctime` (`create_time` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友申请表';

CREATE TABLE IF NOT EXISTS `black` (
    `owner_user_id`    VARCHAR(64)   NOT NULL COMMENT '拉黑者 userID',
    `block_user_id`    VARCHAR(64)   NOT NULL COMMENT '被拉黑者 userID',
    `create_time`      DATETIME      DEFAULT NULL COMMENT '拉黑时间',
    `add_source`       INT           DEFAULT 0 COMMENT '添加来源',
    `operator_user_id` VARCHAR(64)   DEFAULT '' COMMENT '操作者 userID',
    `ex`               VARCHAR(1024) DEFAULT '' COMMENT '扩展字段(json)',
    PRIMARY KEY (`owner_user_id`, `block_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='黑名单表';

-- ============================================================
-- group service
-- ============================================================
CREATE TABLE IF NOT EXISTS `group` (
    `group_id`                  VARCHAR(64)   NOT NULL COMMENT '群组ID',
    `group_name`                VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '群名称',
    `notification`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群公告',
    `introduction`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群简介',
    `face_url`                  VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群头像',
    `create_time`               DATETIME      NOT NULL COMMENT '创建时间',
    `ex`                        VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段',
    `status`                    INT           NOT NULL DEFAULT 0 COMMENT '群状态',
    `creator_user_id`           VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '创建者用户ID',
    `group_type`                INT           NOT NULL DEFAULT 0 COMMENT '群类型',
    `need_verification`         INT           NOT NULL DEFAULT 0 COMMENT '加群是否需要验证',
    `look_member_info`          INT           NOT NULL DEFAULT 0 COMMENT '是否允许查看成员信息',
    `apply_member_friend`       INT           NOT NULL DEFAULT 0 COMMENT '是否允许成员互加好友',
    `notification_update_time`  DATETIME      NOT NULL COMMENT '公告更新时间',
    `notification_user_id`      VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '公告更新者用户ID',
    PRIMARY KEY (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群组信息表';

CREATE TABLE IF NOT EXISTS `group_member` (
    `group_id`         VARCHAR(64)   NOT NULL COMMENT '群组ID',
    `user_id`          VARCHAR(64)   NOT NULL COMMENT '用户ID',
    `nickname`         VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '群内昵称',
    `face_url`         VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '群内头像',
    `role_level`       INT           NOT NULL DEFAULT 0 COMMENT '角色',
    `join_time`        DATETIME      NOT NULL COMMENT '加入时间',
    `join_source`      INT           NOT NULL DEFAULT 0 COMMENT '加入来源',
    `inviter_user_id`  VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '邀请者用户ID',
    `operator_user_id` VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '操作者用户ID',
    `mute_end_time`    DATETIME      NULL COMMENT '禁言结束时间',
    `ex`               VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段',
    PRIMARY KEY (`group_id`, `user_id`),
    INDEX `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群成员表';

CREATE TABLE IF NOT EXISTS `group_request` (
    `user_id`         VARCHAR(64)   NOT NULL COMMENT '申请用户ID',
    `group_id`        VARCHAR(64)   NOT NULL COMMENT '群组ID',
    `handle_result`   INT           NOT NULL DEFAULT 0 COMMENT '处理状态',
    `req_msg`         VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '申请留言',
    `handled_msg`     VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '处理留言',
    `req_time`        DATETIME      NOT NULL COMMENT '申请时间',
    `handle_user_id`  VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '处理者用户ID',
    `handled_time`    DATETIME      NULL COMMENT '处理时间',
    `join_source`     INT           NOT NULL DEFAULT 0 COMMENT '加入来源',
    `inviter_user_id` VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '邀请者用户ID',
    `ex`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段',
    PRIMARY KEY (`group_id`, `user_id`),
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_req_time` (`req_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='加群申请表';

-- ============================================================
-- conversation service
-- ============================================================
CREATE TABLE IF NOT EXISTS `conversation` (
    `owner_user_id`              VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '会话所属用户ID',
    `conversation_id`            VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '会话ID',
    `conversation_type`          INT           NOT NULL DEFAULT 0 COMMENT '会话类型',
    `user_id`                    VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '对端用户ID',
    `group_id`                   VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '群组ID',
    `recv_msg_opt`               INT           NOT NULL DEFAULT 0 COMMENT '消息接收选项',
    `is_pinned`                  TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否置顶',
    `is_private_chat`            TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否私聊',
    `burn_duration`              INT           NOT NULL DEFAULT 0 COMMENT '阅后即焚时长',
    `group_at_type`              INT           NOT NULL DEFAULT 0 COMMENT '群@类型',
    `attached_info`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '附加信息',
    `ex`                         VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段',
    `max_seq`                    BIGINT        NOT NULL DEFAULT 0 COMMENT '最大seq',
    `min_seq`                    BIGINT        NOT NULL DEFAULT 0 COMMENT '最小seq',
    `create_time`                DATETIME      NOT NULL COMMENT '创建时间',
    `is_msg_destruct`            TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否开启消息销毁',
    `msg_destruct_time`          BIGINT        NOT NULL DEFAULT 0 COMMENT '消息销毁时间',
    `latest_msg_destruct_time`   DATETIME      NOT NULL COMMENT '最近消息销毁时间',
    PRIMARY KEY (`owner_user_id`, `conversation_id`),
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_conversation_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户会话表';

-- ============================================================
-- message service
-- ============================================================
CREATE TABLE IF NOT EXISTS `msg_info` (
    `id`                            BIGINT        NOT NULL AUTO_INCREMENT,
    `doc_id`                        VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '文档ID',
    `msg_index`                     INT           NOT NULL DEFAULT 0 COMMENT '在文档中的索引',
    `del_list`                      TEXT                     COMMENT '删除该消息的用户ID列表(json)',
    `is_read`                       TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否已读',
    `conversation_id`               VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '会话ID',
    `send_id`                       VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '发送者用户ID',
    `recv_id`                       VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '接收者用户ID',
    `group_id`                      VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '群组ID',
    `client_msg_id`                 VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '客户端消息ID',
    `server_msg_id`                 VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '服务端消息ID',
    `sender_platform_id`            INT           NOT NULL DEFAULT 0 COMMENT '发送者平台ID',
    `sender_nickname`               VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '发送者昵称',
    `sender_face_url`               VARCHAR(512)  NOT NULL DEFAULT '' COMMENT '发送者头像URL',
    `session_type`                  INT           NOT NULL DEFAULT 0 COMMENT '会话类型',
    `msg_from`                      INT           NOT NULL DEFAULT 0 COMMENT '消息来源',
    `content_type`                  INT           NOT NULL DEFAULT 0 COMMENT '内容类型',
    `content`                       TEXT                     COMMENT '消息内容(json)',
    `seq`                           BIGINT        NOT NULL DEFAULT 0 COMMENT '会话内消息序号',
    `send_time`                     BIGINT        NOT NULL DEFAULT 0 COMMENT '发送时间(Unix毫秒)',
    `create_time`                   BIGINT        NOT NULL DEFAULT 0 COMMENT '创建时间(Unix毫秒)',
    `status`                        INT           NOT NULL DEFAULT 0 COMMENT '消息状态 0=正常 1=已撤回',
    `options`                       TEXT                     COMMENT '消息选项',
    `at_user_id_list`               TEXT                     COMMENT '@提及的用户ID列表',
    `attached_info`                 TEXT                     COMMENT '附加信息',
    `ex`                            TEXT                     COMMENT '扩展字段',
    `offline_push_title`            VARCHAR(255)  DEFAULT NULL COMMENT '离线推送标题',
    `offline_push_desc`             VARCHAR(512)  DEFAULT NULL COMMENT '离线推送描述',
    `offline_push_ex`               TEXT                     COMMENT '离线推送扩展',
    `offline_push_ios_sound`        VARCHAR(255)  DEFAULT NULL COMMENT 'iOS推送声音',
    `offline_push_ios_badge_count`  TINYINT(1)    NOT NULL DEFAULT 0 COMMENT 'iOS角标',
    `revoke_role`                   INT           DEFAULT NULL COMMENT '撤回者角色',
    `revoke_user_id`                VARCHAR(64)   DEFAULT NULL COMMENT '撤回者用户ID',
    `revoke_nickname`               VARCHAR(255)  DEFAULT NULL COMMENT '撤回者昵称',
    `revoke_time`                   BIGINT        DEFAULT NULL COMMENT '撤回时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_server_msg_id` (`server_msg_id`),
    INDEX `idx_doc_id` (`doc_id`),
    INDEX `idx_conversation_id` (`conversation_id`),
    INDEX `idx_send_id` (`send_id`),
    INDEX `idx_recv_id` (`recv_id`),
    INDEX `idx_group_id` (`group_id`),
    INDEX `idx_seq` (`seq`),
    INDEX `idx_send_time` (`send_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消息表';

CREATE TABLE IF NOT EXISTS `seq_conversation` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `conversation_id` VARCHAR(255) NOT NULL COMMENT '会话ID',
    `max_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '当前最大消息序号',
    `min_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '当前最小消息序号',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_conversation_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话序号表';

CREATE TABLE IF NOT EXISTS `seq_user` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`         VARCHAR(64)  NOT NULL COMMENT '用户ID',
    `conversation_id` VARCHAR(255) NOT NULL COMMENT '会话ID',
    `min_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '最小可见序号',
    `max_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '最大可见序号',
    `read_seq`        BIGINT       NOT NULL DEFAULT 0 COMMENT '已读序号',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_user_conversation` (`user_id`, `conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户会话序号表';

-- ============================================================
-- push service
-- ============================================================
CREATE TABLE IF NOT EXISTS `push_token` (
    `id`          BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id`     VARCHAR(64)  NOT NULL COMMENT '用户ID',
    `platform_id` INT          NOT NULL DEFAULT 0 COMMENT '平台ID',
    `token`       VARCHAR(512) NOT NULL COMMENT '设备推送令牌',
    `created_at`  BIGINT       NOT NULL DEFAULT 0 COMMENT '创建时间',
    `updated_at`  BIGINT       NOT NULL DEFAULT 0 COMMENT '更新时间',
    UNIQUE KEY `idx_user_platform` (`user_id`, `platform_id`),
    INDEX `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户设备推送令牌表';

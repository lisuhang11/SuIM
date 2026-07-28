-- ============================================================
-- 消息存储 (OpenIM MsgDocModel / MsgInfoModel 的关系型等价实现)
-- ============================================================

-- 每条消息一行 (对应 OpenIM msg collection 展开)
CREATE TABLE IF NOT EXISTS `msg_info` (
    `id`                  BIGINT        NOT NULL AUTO_INCREMENT,
    `doc_id`              VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '文档ID, 格式: {conversationID}:{docIndex}',
    `msg_index`           INT           NOT NULL DEFAULT 0 COMMENT '在文档中的索引位置(0-99)',
    `del_list`            TEXT                     COMMENT '删除该消息的用户ID列表(json)',
    `is_read`             TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否已读',

    `conversation_id`     VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '会话ID(冗余列, 便于索引查询)',

    -- === MsgDataModel ===
    `send_id`             VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '发送者用户ID',
    `recv_id`             VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '接收者用户ID(单聊)',
    `group_id`            VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '群组ID(群聊)',
    `client_msg_id`       VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '客户端消息ID',
    `server_msg_id`       VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '服务端消息ID',
    `sender_platform_id`  INT           NOT NULL DEFAULT 0 COMMENT '发送者平台ID',
    `sender_nickname`     VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '发送者昵称',
    `sender_face_url`     VARCHAR(512)  NOT NULL DEFAULT '' COMMENT '发送者头像URL',
    `session_type`        INT           NOT NULL DEFAULT 0 COMMENT '会话类型(1:单聊 2:群聊)',
    `msg_from`            INT           NOT NULL DEFAULT 0 COMMENT '消息来源',
    `content_type`        INT           NOT NULL DEFAULT 0 COMMENT '内容类型',
    `content`             TEXT                     COMMENT '消息内容(json)',
    `seq`                 BIGINT        NOT NULL DEFAULT 0 COMMENT '会话内消息序号',
    `send_time`           BIGINT        NOT NULL DEFAULT 0 COMMENT '发送时间(Unix毫秒)',
    `create_time`         BIGINT        NOT NULL DEFAULT 0 COMMENT '创建时间(Unix毫秒)',
    `status`              INT           NOT NULL DEFAULT 0 COMMENT '消息状态 0=正常 1=已撤回',
    `options`             TEXT                     COMMENT '消息选项(json map[string]bool)',
    `at_user_id_list`     TEXT                     COMMENT '@提及的用户ID列表(json)',
    `attached_info`       TEXT                     COMMENT '附加信息',
    `ex`                  TEXT                     COMMENT '扩展字段',

    -- === OfflinePushModel ===
    `offline_push_title`            VARCHAR(255) DEFAULT NULL COMMENT '离线推送标题',
    `offline_push_desc`             VARCHAR(512) DEFAULT NULL COMMENT '离线推送描述',
    `offline_push_ex`               TEXT                     COMMENT '离线推送扩展',
    `offline_push_ios_sound`        VARCHAR(255) DEFAULT NULL COMMENT 'iOS推送声音',
    `offline_push_ios_badge_count`  TINYINT(1)    NOT NULL DEFAULT 0 COMMENT 'iOS角标',

    -- === RevokeModel ===
    `revoke_role`         INT           DEFAULT NULL COMMENT '撤回者角色',
    `revoke_user_id`      VARCHAR(64)   DEFAULT NULL COMMENT '撤回者用户ID',
    `revoke_nickname`     VARCHAR(255)  DEFAULT NULL COMMENT '撤回者昵称',
    `revoke_time`         BIGINT        DEFAULT NULL COMMENT '撤回时间',

    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_server_msg_id` (`server_msg_id`),
    KEY `idx_doc_id` (`doc_id`),
    KEY `idx_conversation_id` (`conversation_id`),
    KEY `idx_send_id` (`send_id`),
    KEY `idx_recv_id` (`recv_id`),
    KEY `idx_group_id` (`group_id`),
    KEY `idx_seq` (`seq`),
    KEY `idx_send_time` (`send_time`),
    KEY `idx_content_type` (`content_type`),
    KEY `idx_session_type` (`session_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消息表(每条消息一行)';

-- ============================================================
-- 会话序号表 (OpenIM seq collection)
-- ============================================================
CREATE TABLE IF NOT EXISTS `seq_conversation` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `conversation_id` VARCHAR(255) NOT NULL COMMENT '会话ID',
    `max_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '当前最大消息序号',
    `min_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '当前最小消息序号',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_conversation_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话序号表';

-- ============================================================
-- 用户会话序号表 (OpenIM seq_user collection)
-- ============================================================
CREATE TABLE IF NOT EXISTS `seq_user` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`         VARCHAR(64)  NOT NULL COMMENT '用户ID',
    `conversation_id` VARCHAR(255) NOT NULL COMMENT '会话ID',
    `min_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '该用户在此会话的最小可见序号',
    `max_seq`         BIGINT       NOT NULL DEFAULT 0 COMMENT '该用户在此会话的最大可见序号',
    `read_seq`        BIGINT       NOT NULL DEFAULT 0 COMMENT '该用户在此会话的已读序号',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_user_conversation` (`user_id`, `conversation_id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_conversation_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户会话序号表';

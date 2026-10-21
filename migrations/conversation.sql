CREATE TABLE `conversation` (
  `owner_user_id`            VARCHAR(64)  NOT NULL COMMENT '会话拥有者',
  `conversation_id`          VARCHAR(255) NOT NULL COMMENT '会话ID',
  `conversation_type`        INT          NOT NULL DEFAULT 0 COMMENT '会话类型',
  `user_id`                  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '单聊对端用户',
  `group_id`                 VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '群ID',
  `recv_msg_opt`             INT          NOT NULL DEFAULT 0 COMMENT '消息接收选项',
  `is_pinned`                TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否置顶，1是0否',
  `is_private_chat`          TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否阅后即焚，1是0否',
  `burn_duration`            INT          NOT NULL DEFAULT 0 COMMENT '阅后即焚时长，单位秒',
  `group_at_type`            INT          NOT NULL DEFAULT 0 COMMENT '群@类型',
  `attached_info`            TEXT         NULL COMMENT '附加信息',
  `max_seq`                  BIGINT       NOT NULL DEFAULT 0 COMMENT '最大消息序号',
  `min_seq`                  BIGINT       NOT NULL DEFAULT 0 COMMENT '最小消息序号',
  `create_time`              DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  `is_msg_destruct`          TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否定时销毁消息，1是0否',
  `msg_destruct_time`        BIGINT       NOT NULL DEFAULT 0 COMMENT '消息销毁时长',
  `latest_msg_destruct_time` DATETIME(3)  NULL COMMENT '最近一次销毁时间',
  PRIMARY KEY (`owner_user_id`, `conversation_id`),
  KEY `idx_conversation_user_id` (`user_id`),
  KEY `idx_conversation_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话';

-- 每个用户一行。version_id 返回给客户端，文档重建后必须换新值。
CREATE TABLE `conversation_version` (
  `version_id`  CHAR(24)        NOT NULL COMMENT '版本ID，文档重建后更换',
  `d_id`        VARCHAR(64)     NOT NULL COMMENT '会话拥有者',
  `version`     BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '当前版本号',
  `deleted`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '固定为0',
  `last_update` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '最后更新时间',
  PRIMARY KEY (`d_id`),
  UNIQUE KEY `uk_conversation_version_id` (`version_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话版本';

-- 每个会话只保留最新一条变更。
CREATE TABLE `conversation_version_log` (
  `d_id`        VARCHAR(64)     NOT NULL COMMENT '会话拥有者',
  `e_id`        VARCHAR(255)    NOT NULL COMMENT '会话ID',
  `state`       INT             NOT NULL COMMENT '1插入 2删除 3更新',
  `version`     BIGINT UNSIGNED NOT NULL COMMENT '该元素的版本号',
  `last_update` DATETIME(3)     NOT NULL COMMENT '最后更新时间',
  PRIMARY KEY (`d_id`, `e_id`),
  KEY `idx_conversation_version_log_version` (`d_id`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话版本明细';

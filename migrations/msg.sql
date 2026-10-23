CREATE TABLE IF NOT EXISTS `msg` (
  `conversation_id`       VARCHAR(128) NOT NULL,
  `seq`                   BIGINT       NOT NULL,
  `send_id`               VARCHAR(64)  NOT NULL DEFAULT '',
  `recv_id`               VARCHAR(64)  NOT NULL DEFAULT '',
  `group_id`              VARCHAR(64)  NOT NULL DEFAULT '',
  `client_msg_id`         VARCHAR(64)  NOT NULL DEFAULT '',
  `server_msg_id`         VARCHAR(64)  NOT NULL DEFAULT '',
  `sender_platform_id`    INT          NOT NULL DEFAULT 0,
  `sender_nickname`       VARCHAR(255) NOT NULL DEFAULT '',
  `sender_face_url`       VARCHAR(512) NOT NULL DEFAULT '',
  `session_type`          INT          NOT NULL DEFAULT 0,
  `msg_from`              INT          NOT NULL DEFAULT 0,
  `content_type`          INT          NOT NULL DEFAULT 0,
  `content`               MEDIUMTEXT   NOT NULL,
  `send_time`             BIGINT       NOT NULL DEFAULT 0,
  `create_time`           BIGINT       NOT NULL DEFAULT 0,
  `status`                INT          NOT NULL DEFAULT 0,
  `is_read`               TINYINT(1)   NOT NULL DEFAULT 0,
  `options`               JSON         NULL,
  `at_user_id_list`       JSON         NULL,
  `attached_info`         TEXT         NULL,
  `ex`                    TEXT         NULL,
  `offline_push_title`    VARCHAR(255) NOT NULL DEFAULT '',
  `offline_push_desc`     VARCHAR(1024) NOT NULL DEFAULT '',
  `offline_push_ex`       VARCHAR(1024) NOT NULL DEFAULT '',
  `offline_push_ios_sound` VARCHAR(255) NOT NULL DEFAULT '',
  `offline_push_ios_badge_count` TINYINT(1) NOT NULL DEFAULT 0,
  `revoke_role`           INT          NULL,
  `revoke_user_id`        VARCHAR(64)  NULL,
  `revoke_nickname`       VARCHAR(255) NULL,
  `revoke_time`           BIGINT       NULL,
  PRIMARY KEY (`conversation_id`, `seq`),
  KEY `idx_server_msg_id` (`server_msg_id`),
  KEY `idx_client_msg_id` (`client_msg_id`),
  KEY `idx_send_time` (`conversation_id`, `send_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `msg_user_delete` (
  `conversation_id` VARCHAR(128) NOT NULL,
  `seq`             BIGINT       NOT NULL,
  `user_id`         VARCHAR(64)  NOT NULL,
  PRIMARY KEY (`conversation_id`, `seq`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `seq_conversation` (
  `conversation_id` VARCHAR(128) NOT NULL,
  `max_seq`         BIGINT       NOT NULL DEFAULT 0,
  `min_seq`         BIGINT       NOT NULL DEFAULT 0,
  PRIMARY KEY (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `seq_user` (
  `user_id`         VARCHAR(64)  NOT NULL,
  `conversation_id` VARCHAR(128) NOT NULL,
  `min_seq`         BIGINT       NOT NULL DEFAULT 0,
  `max_seq`         BIGINT       NOT NULL DEFAULT 0,
  `read_seq`        BIGINT       NOT NULL DEFAULT 0,
  PRIMARY KEY (`user_id`, `conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
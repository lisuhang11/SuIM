CREATE TABLE `friend` (
                          `owner_user_id`   VARCHAR(64)   NOT NULL COMMENT '关系拥有者 userID',
                          `friend_user_id`  VARCHAR(64)   NOT NULL COMMENT '好友 userID',
                          `remark`          VARCHAR(255)  DEFAULT '' COMMENT '好友备注',
                          `create_time`     DATETIME      DEFAULT NULL COMMENT '成为好友的时间',
                          `add_source`      INT           DEFAULT 0 COMMENT '添加来源',
                          `operator_user_id` VARCHAR(64)  DEFAULT '' COMMENT '操作者 userID',
                          `ex`              VARCHAR(1024) DEFAULT '' COMMENT '扩展字段(json)',
                          `is_pinned`       TINYINT(1)    DEFAULT 0 COMMENT '是否置顶(1是0否)',
                          PRIMARY KEY (`owner_user_id`, `friend_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友关系表(单方视角)';

-- MongoDB 唯一索引 (owner_user_id, friend_user_id) 的等价
-- 上述 PRIMARY KEY 已保证唯一
-- MongoDB 查询排序 {is_pinned:-1, _id:1} 的等价辅助索引
CREATE INDEX `idx_friend_owner_pinned` ON `friend` (`owner_user_id`, `is_pinned` DESC, `create_time`);

CREATE TABLE `friend_request` (
                                  `from_user_id`    VARCHAR(64) NOT NULL COMMENT '申请人 userID',
                                  `to_user_id`      VARCHAR(64) NOT NULL COMMENT '被申请人 userID',
                                  `handle_result`   INT         DEFAULT 0 COMMENT '0未处理 1同意 -1拒绝',
                                  `req_msg`         VARCHAR(512) DEFAULT '' COMMENT '申请留言',
                                  `create_time`     DATETIME    DEFAULT NULL COMMENT '申请时间',
                                  `handler_user_id` VARCHAR(64) DEFAULT '' COMMENT '处理人 userID',
                                  `handle_msg`      VARCHAR(512) DEFAULT '' COMMENT '处理留言',
                                  `handle_time`     DATETIME    DEFAULT NULL COMMENT '处理时间',
                                  `ex`              VARCHAR(1024) DEFAULT '' COMMENT '扩展字段(json)',
                                  PRIMARY KEY (`from_user_id`, `to_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友申请表';

-- MongoDB 唯一索引 (from_user_id, to_user_id)
-- 上述 PRIMARY KEY 已保证唯一
-- MongoDB 普通索引 {create_time:-1} (分页按时间倒序)
CREATE INDEX `idx_friend_request_ctime` ON `friend_request` (`create_time` DESC);


CREATE TABLE `black` (
                         `owner_user_id`    VARCHAR(64) NOT NULL COMMENT '拉黑者 userID',
                         `block_user_id`    VARCHAR(64) NOT NULL COMMENT '被拉黑者 userID',
                         `create_time`      DATETIME    DEFAULT NULL COMMENT '拉黑时间',
                         `add_source`       INT         DEFAULT 0 COMMENT '添加来源',
                         `operator_user_id` VARCHAR(64) DEFAULT '' COMMENT '操作者 userID',
                         `ex`               VARCHAR(1024) DEFAULT '' COMMENT '扩展字段(json)',
                         PRIMARY KEY (`owner_user_id`, `block_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='黑名单表(单方视角)';

-- MongoDB 唯一索引 (owner_user_id, block_user_id) 已由 PRIMARY KEY 保证

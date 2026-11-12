package cron

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"google.golang.org/grpc"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	svcconfig "github.com/lisuhang11/SuIM/services/crontask/internal/config"
)

type Server struct {
	ctx                context.Context
	config             *svcconfig.Config
	cron               *cron.Cron
	msgClient          msgpb.MsgServiceClient
	conversationClient conversationpb.ConversationServiceClient
	thirdClient        thirdpb.ThirdServiceClient
	locker             Locker
}

func Start(ctx context.Context, conf *svcconfig.Config, msgConn, thirdConn, conversationConn *grpc.ClientConn, locker Locker) error {
	if conf.CronTask.RetainChatRecords < 1 &&
		(conf.CronTask.FileExpireTime <= 0 || len(conf.CronTask.DeleteObjectType) == 0) {
		slog.Info("crontask disabled: retainChatRecords < 1 and object cleanup off")
		<-ctx.Done()
		return nil
	}

	srv := &Server{
		ctx:                ctx,
		config:             conf,
		cron:               cron.New(),
		msgClient:          msgpb.NewMsgServiceClient(msgConn),
		conversationClient: conversationpb.NewConversationServiceClient(conversationConn),
		thirdClient:        thirdpb.NewThirdServiceClient(thirdConn),
		locker:             locker,
	}
	if err := srv.registerClearS3(); err != nil {
		return err
	}
	if err := srv.registerDeleteMsg(); err != nil {
		return err
	}
	if err := srv.registerClearUserMsg(); err != nil {
		return err
	}
	slog.Info("start cron task", "cronExecuteTime", conf.CronTask.CronExecuteTime)
	srv.cron.Start()
	<-ctx.Done()
	stopCtx := srv.cron.Stop()
	<-stopCtx.Done()
	return nil
}

func (c *Server) registerClearS3() error {
	if c.config.CronTask.FileExpireTime <= 0 || len(c.config.CronTask.DeleteObjectType) == 0 {
		slog.Info("disable scheduled cleanup of s3",
			"fileExpireTime", c.config.CronTask.FileExpireTime,
			"deleteObjectType", c.config.CronTask.DeleteObjectType)
		return nil
	}
	_, err := c.cron.AddFunc(c.config.CronTask.CronExecuteTime, func() {
		c.locker.ExecuteWithLock(c.ctx, "clearS3", c.clearS3)
	})
	if err != nil {
		return fmt.Errorf("register clear s3 cron: %w", err)
	}
	return nil
}

func (c *Server) registerDeleteMsg() error {
	if c.config.CronTask.RetainChatRecords <= 0 {
		slog.Info("disable scheduled cleanup of chat records",
			"retainChatRecords", c.config.CronTask.RetainChatRecords)
		return nil
	}
	_, err := c.cron.AddFunc(c.config.CronTask.CronExecuteTime, func() {
		c.locker.ExecuteWithLock(c.ctx, "deleteMsg", c.deleteMsg)
	})
	if err != nil {
		return fmt.Errorf("register delete msg cron: %w", err)
	}
	return nil
}

func (c *Server) registerClearUserMsg() error {
	_, err := c.cron.AddFunc(c.config.CronTask.CronExecuteTime, func() {
		c.locker.ExecuteWithLock(c.ctx, "clearUserMsg", c.clearUserMsg)
	})
	if err != nil {
		return fmt.Errorf("register clear user msg cron: %w", err)
	}
	return nil
}

func (c *Server) deleteMsg() {
	now := time.Now()
	deltime := now.Add(-time.Hour * 24 * time.Duration(c.config.CronTask.RetainChatRecords))
	operationID := fmt.Sprintf("cron_msg_%d_%d", os.Getpid(), deltime.UnixMilli())
	const (
		deleteCount = 10000
		deleteLimit = 50
	)
	var count int
	for i := 1; i <= deleteCount; i++ {
		resp, err := c.msgClient.DestructMsgs(c.ctx, &msgpb.DestructMsgsReq{
			Timestamp: deltime.UnixMilli(),
			Limit:     deleteLimit,
		})
		if err != nil {
			slog.Error("cron destruct chat records failed", "operationID", operationID, "err", err)
			break
		}
		count += int(resp.GetCount())
		if resp.GetCount() < deleteLimit {
			break
		}
	}
	slog.Info("cron destruct chat records end",
		"operationID", operationID, "deltime", deltime, "count", count, "cost", time.Since(now))
}

func (c *Server) clearS3() {
	start := time.Now()
	deleteTime := start.Add(-time.Hour * 24 * time.Duration(c.config.CronTask.FileExpireTime))
	operationID := fmt.Sprintf("cron_s3_%d_%d", os.Getpid(), deleteTime.UnixMilli())
	const (
		deleteCount = 10000
		deleteLimit = 100
	)
	var count int
	for i := 1; i <= deleteCount; i++ {
		resp, err := c.thirdClient.DeleteOutdatedData(c.ctx, &thirdpb.DeleteOutdatedDataReq{
			ExpireTime:  deleteTime.UnixMilli(),
			ObjectGroup: c.config.CronTask.DeleteObjectType,
			Limit:       deleteLimit,
		})
		if err != nil {
			slog.Error("cron delete outdated data failed", "operationID", operationID, "err", err)
			return
		}
		count += int(resp.GetCount())
		if resp.GetCount() < deleteLimit {
			break
		}
	}
	slog.Info("cron delete outdated data success",
		"operationID", operationID, "deltime", deleteTime, "count", count, "cost", time.Since(start))
}

func (c *Server) clearUserMsg() {
	now := time.Now()
	operationID := fmt.Sprintf("cron_user_msg_%d_%d", os.Getpid(), now.UnixMilli())
	const (
		deleteCount = 10000
		deleteLimit = 100
	)
	var count int
	for i := 1; i <= deleteCount; i++ {
		resp, err := c.conversationClient.ClearUserConversationMsg(c.ctx, &conversationpb.ClearUserConversationMsgReq{
			Timestamp: now.UnixMilli(),
			Limit:     deleteLimit,
		})
		if err != nil {
			slog.Error("ClearUserConversationMsg failed", "operationID", operationID, "err", err)
			return
		}
		count += int(resp.GetCount())
		if resp.GetCount() < deleteLimit {
			break
		}
	}
	slog.Info("clear user msg cron completed",
		"operationID", operationID, "count", count, "cost", time.Since(now))
}

package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/ent/paymentaccount"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mysql"

	"go.uber.org/fx"
)

const (
	// AppHeartbeatInterval 与 App 端固定心跳间隔对齐。
	AppHeartbeatInterval = 10 * time.Second
	// AppHeartbeatMissLimit 连续丢失次数阈值。
	AppHeartbeatMissLimit = 3
	// AppOnlineWindow 控制台「在线」判定窗口（略大于 3 次心跳）。
	AppOnlineWindow = AppHeartbeatInterval*AppHeartbeatMissLimit + 5*time.Second
)

// HeartbeatWatchService 定时扫描设备心跳，超过 3 次未上报则发 IM 告警。
type HeartbeatWatchService struct {
	db     *mysql.Client
	bot    *BotNotifyService
	log    *logger.Logger
	mu     sync.Mutex
	alerted map[int64]time.Time // accountID -> 已告警时间
}

func NewHeartbeatWatchService(db *mysql.Client, bot *BotNotifyService, log *logger.Logger) *HeartbeatWatchService {
	return &HeartbeatWatchService{
		db:      db,
		bot:     bot,
		log:     log,
		alerted: make(map[int64]time.Time),
	}
}

func RegisterHeartbeatWatch(lc fx.Lifecycle, s *HeartbeatWatchService) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go s.loop(ctx)
			s.log.Info(ctx, "heartbeat watch started",
				"interval", AppHeartbeatInterval.String(),
				"miss_limit", AppHeartbeatMissLimit)
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
}

func (s *HeartbeatWatchService) loop(ctx context.Context) {
	ticker := time.NewTicker(AppHeartbeatInterval)
	defer ticker.Stop()
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *HeartbeatWatchService) tick(ctx context.Context) {
	now := time.Now()
	offlineBefore := now.Add(-AppHeartbeatInterval * AppHeartbeatMissLimit)
	// 只关心近期曾在线的设备，避免从未心跳的账号刷告警
	recentSince := now.Add(-24 * time.Hour)
	list, err := s.db.Ent().PaymentAccount.Query().
		Where(
			paymentaccount.StatusEQ("active"),
			paymentaccount.AppLastSeenAtNotNil(),
			paymentaccount.AppLastSeenAtGTE(recentSince),
		).
		All(ctx)
	if err != nil {
		s.log.Warn(ctx, "heartbeat watch query failed", "err", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	live := make(map[int64]struct{}, len(list))
	for _, acc := range list {
		live[acc.ID] = struct{}{}
		if acc.AppLastSeenAt == nil {
			continue
		}
		if acc.AppLastSeenAt.After(offlineBefore) {
			// 恢复在线：清除告警标记
			delete(s.alerted, acc.ID)
			continue
		}
		if _, already := s.alerted[acc.ID]; already {
			continue
		}
		missed := int(now.Sub(*acc.AppLastSeenAt) / AppHeartbeatInterval)
		if missed < AppHeartbeatMissLimit {
			continue
		}
		s.alerted[acc.ID] = now
		accCopy := *acc
		go s.bot.EnqueueDeviceOffline(&accCopy, missed)
		s.log.Warn(ctx, "device heartbeat lost",
			"account_id", acc.ID,
			"user_id", acc.UserID,
			"callback_id", acc.CallbackID,
			"missed", missed,
			"last_seen", acc.AppLastSeenAt)
	}
	// 清理已删除/停用设备的告警缓存
	for id := range s.alerted {
		if _, ok := live[id]; !ok {
			delete(s.alerted, id)
		}
	}
}

// MarkOnline 心跳成功时清除告警态（可选即时调用）。
func (s *HeartbeatWatchService) MarkOnline(accountID int64) {
	s.mu.Lock()
	delete(s.alerted, accountID)
	s.mu.Unlock()
}

func (s *BotNotifyService) EnqueueDeviceOffline(acc *ent.PaymentAccount, missed int) {
	if acc == nil {
		return
	}
	name := acc.Name
	if name == "" {
		name = acc.CallbackID
	}
	lastSeen := "—"
	if acc.AppLastSeenAt != nil {
		lastSeen = acc.AppLastSeenAt.Local().Format("2006-01-02 15:04:05")
	}
	conn := "—"
	if acc.AppConnType != nil && *acc.AppConnType != "" {
		conn = *acc.AppConnType
		if conn == "ws" {
			conn = "WebSocket"
		}
	}
	ev := BotEvent{
		Kind:      NotifyEventDeviceOffline,
		Title:     "监控设备心跳丢失",
		Summary:   fmt.Sprintf("设备「%s」已连续超过 %d 次未心跳", name, AppHeartbeatMissLimit),
		Level:     "warning",
		IdemKey:   fmt.Sprintf("device_offline:%d:%d", acc.ID, time.Now().Unix()/60),
		AccountID: acc.ID,
		Fields: []BotField{
			{Label: "设备", Value: name},
			{Label: "Callback", Value: acc.CallbackID},
			{Label: "连接", Value: conn},
			{Label: "最近心跳", Value: lastSeen},
			{Label: "估算丢失", Value: fmt.Sprintf("%d 次（间隔 %s）", missed, AppHeartbeatInterval)},
		},
	}
	go s.dispatch(acc.UserID, NotifyEventDeviceOffline, ev)
}

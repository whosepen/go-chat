package service

import (
	"context"
	"encoding/json"
	"go-chat/global"
	"go-chat/internal/models"
	"time"

	"go.uber.org/zap"
)

const (
	PersistQueue  = "chat:msg:queue"
	BatchSize     = 500                   // 提高 Batch Size，减少 IO 次数
	FlushInterval = 500 * time.Millisecond // 缩短兜底刷新时间
	WriteInterval = 50 * time.Millisecond  // 【关键】两次写入之间的强制最小间隔 (削峰)
)

// StartBatchPersister 启动批量持久化协程
// 采用 "Single Worker" 模式：无论 Redis 积压多少消息，永远只有一个协程在匀速写入 DB
// 数据库压力 = BatchSize / WriteInterval (例如 500条 / 50ms = 10,000 TPS 上限)
func StartBatchPersister() {
	go func() {
		global.Log.Info("Starting Batch Persister (Rate Limited)...")

		// 预分配内存，避免反复扩容
		buffer := make([]models.Message, 0, BatchSize)
		
		// 兜底定时器
		ticker := time.NewTicker(FlushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 定时强制刷盘 (处理不够 BatchSize 的尾部数据)
				if len(buffer) > 0 {
					flushBuffer(&buffer)
				}
			default:
				// 阻塞读取 Redis 队列
				// 使用较短的超时时间，保证能及时响应 ticker
				result, err := global.RDB.BRPop(context.Background(), 200*time.Millisecond, PersistQueue).Result()
				
				if err == nil {
					// 成功取到数据
					var msg models.Message
					if err := json.Unmarshal([]byte(result[1]), &msg); err == nil {
						buffer = append(buffer, msg)
					} else {
						global.Log.Error("unmarshal msg failed", zap.Error(err))
					}
				}

				// 只要 buffer 有数据，且满足以下条件之一就刷盘：
				// 1. 达到 BatchSize
				// 2. (隐含) ticker 触发了 (上面 case 处理)
				if len(buffer) >= BatchSize {
					flushBuffer(&buffer)
					// 【关键】写完一批后，强制休眠一小会儿，防止瞬间打死数据库
					time.Sleep(WriteInterval)
				}
			}
		}
	}()
}

func flushBuffer(buffer *[]models.Message) {
	if len(*buffer) == 0 {
		return
	}

	start := time.Now()
	count := len(*buffer)

	// 批量插入
	err := global.DB.Create(buffer).Error
	
	cost := time.Since(start)
	
	if err != nil {
		global.Log.Error("Batch insert failed", 
			zap.Error(err), 
			zap.Int("count", count),
			zap.Duration("cost", cost),
		)
		// 失败重试逻辑 (简单将失败消息推回 Redis 队尾，防止丢数据)
		// 注意：生产环境建议推送到 Dead Letter Queue (死信队列)
		go func(failedMsgs []models.Message) {
			ctx := context.Background()
			pipe := global.RDB.Pipeline()
			for _, msg := range failedMsgs {
				bytes, _ := json.Marshal(msg)
				pipe.RPush(ctx, PersistQueue, bytes) // RPush 放回队尾
			}
			pipe.Exec(ctx)
		}(*buffer)
	} else {
		// 正常情况下不打印 Info 日志，避免日志刷屏，只记录慢查询
		if cost > 100*time.Millisecond {
			global.Log.Warn("Slow batch insert", zap.Int("count", count), zap.Duration("cost", cost))
		}
	}

	// 重置 buffer (复用底层数组容量)
	*buffer = (*buffer)[:0]
}
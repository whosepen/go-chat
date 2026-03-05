package service

import (
	"context"
	"encoding/json"
	"errors"
	"go-chat/global"
	"go-chat/internal/models"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9" // Import redis
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	BatchSize     = 500
	FlushInterval = 500 * time.Millisecond
)

func StartConsumer() {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	// 使用 Sticky 分区策略，减少重平衡
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategySticky()

	// 获取 Consumer Group ID
	groupID := global.Config.GetString("kafka.consumer_group")
	if groupID == "" {
		groupID = "chat_group"
	}

	client, err := sarama.NewConsumerGroup(global.KAdrrs, groupID, config)
	if err != nil {
		global.Log.Fatal("start consumer group failed", zap.Error(err))
	}

	// 监听错误
	go func() {
		for err := range client.Errors() {
			global.Log.Error("consumer group error", zap.Error(err))
		}
	}()

	// 启动消费者循环
	// 注意：Consume 是阻塞的，需要在一个 goroutine 中运行
	// 这里我们需要消费多个 Topic
	topics := []string{
		global.KTopic.ChatMsg, // 新增：监听私聊
		global.KTopic.GroupMsg,
		global.KTopic.Retry,
		global.KTopic.Dead,
	}

	// 定义 Handler
	handler := &BatchConsumerGroupHandler{
		Handlers: map[string]func([]*sarama.ConsumerMessage) error{
			global.KTopic.ChatMsg:  handlePrivateMessageBatch, // 批量处理私聊
			global.KTopic.GroupMsg: handleGroupMessageBatch,   // 批量处理群聊
			global.KTopic.Retry:    handleRetryBatch,          // 批量处理重试
			global.KTopic.Dead:     handleDeadLetterBatch,
		},
	}

	go func() {
		ctx := context.Background()
		for {
			// Consume 会在 rebalance 时返回，所以需要循环调用
			if err := client.Consume(ctx, topics, handler); err != nil {
				global.Log.Error("Error from consumer", zap.Error(err))
				time.Sleep(time.Second) // 防止死循环空转
			}
		}
	}()

	global.Log.Info("Consumer Group started (Batch Mode)", zap.Strings("topics", topics), zap.String("group", groupID))
}

// BatchConsumerGroupHandler 实现 sarama.ConsumerGroupHandler 接口，支持批量消费
type BatchConsumerGroupHandler struct {
	Handlers map[string]func([]*sarama.ConsumerMessage) error
}

func (h *BatchConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *BatchConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *BatchConsumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	handler, ok := h.Handlers[claim.Topic()]
	if !ok {
		return nil
	}

	var batch []*sarama.ConsumerMessage
	ticker := time.NewTicker(FlushInterval)
	defer ticker.Stop()

	// flush 核心逻辑：先存储，后确认
	flush := func() {
		if len(batch) == 0 {
			return
		}
		// 1. 执行批量业务处理 (落库)
		// 注意：如果这里返回 error，说明 DB 写入失败，我们不能标记 Offset
		if err := handler(batch); err == nil {
			// 2. 只有业务处理成功，才向 Kafka 确认 Offset
			// 标记最后一条消息即可，代表之前的都消费了
			sess.MarkMessage(batch[len(batch)-1], "")
		} else {
			global.Log.Error("Batch handler failed, offset NOT marked", zap.Error(err))
		}
		// 清空 batch，准备下一轮
		batch = batch[:0]
	}

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				flush() // Channel 关闭前最后一次 flush
				return nil
			}
			batch = append(batch, msg)
			if len(batch) >= BatchSize {
				flush()
			}
		case <-ticker.C:
			flush() // 定时 flush，防止低频消息时延迟过高
		case <-sess.Context().Done():
			return nil
		}
	}
}

// ------ handlePrivateMessageBatch 批量处理私聊消息 ------------------------------------------------------
func handlePrivateMessageBatch(msgs []*sarama.ConsumerMessage) error {
	if len(msgs) == 0 {
		return nil
	}

	var dbMsgs []*models.Message
	var validMsgs []*sarama.ConsumerMessage

	// 1. 解析所有消息
	for _, msg := range msgs {
		var dbMsg models.Message
		if err := json.Unmarshal(msg.Value, &dbMsg); err != nil {
			global.Log.Error("Unmarshal failed, sending to Dead Letter", zap.Error(err))
			republish(global.KTopic.Dead, msg.Value)
			continue
		}
		dbMsgs = append(dbMsgs, &dbMsg)
		validMsgs = append(validMsgs, msg)
	}

	if len(dbMsgs) == 0 {
		return nil
	}

	// 2. 批量写入 MySQL (核心优化)
	// 使用 INSERT IGNORE / ON CONFLICT DO NOTHING 保证幂等性
	// 即使 Kafka 重复投递，DB 也会自动忽略，不会报错
	err := global.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "msg_id"}}, // 依据 msg_id 去重
		DoNothing: true,
	}).Create(&dbMsgs).Error

	if err != nil {
		global.Log.Error("Batch insert private messages failed, fallback to single insert", zap.Error(err))
		// 降级策略：如果批量失败，尝试单条逐个处理，尽可能挽救
		for i, dbMsg := range dbMsgs {
			if err := handleSinglePrivateMessage(dbMsg, validMsgs[i].Value); err != nil {
				continue
			}
		}
		// 这里返回 nil 是因为我们已经进行了降级处理。
		// 如果降级处理也全失败了，handleSinglePrivateMessage 内部会投递到重试队列。
		// 所以对于 Kafka 来说，这批消息算是“处理过”了（哪怕是处理成了重试状态）。
		return nil
	}

	// 3. 推送通知
	// 乐观推送模式：API Server 已推送，Consumer 不再推送，避免重复
	// for _, dbMsg := range dbMsgs {
	// 	PublishPushConsumer(*dbMsg, false)
	// }

	return nil
}

// 辅助函数：单条处理私聊消息 (用于降级)
func handleSinglePrivateMessage(dbMsg *models.Message, value []byte) error {
	for i := 0; i < global.RetryMax; i++ {
		err := global.DB.Create(dbMsg).Error
		if err == nil || errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	global.Log.Warn("Private message retry failed", zap.Any("msg", dbMsg))
	republish(global.KTopic.Retry, value)
	return nil
}

// ------ handleGroupMessageBatch 批量处理群聊消息 --------------------------------------------------------
func handleGroupMessageBatch(msgs []*sarama.ConsumerMessage) error {
	if len(msgs) == 0 {
		return nil
	}

	var dbMsgs []*models.Message
	var validMsgs []*sarama.ConsumerMessage

	// 1. 解析
	for _, msg := range msgs {
		var dbMsg models.Message
		if err := json.Unmarshal(msg.Value, &dbMsg); err != nil {
			republish(global.KTopic.Dead, msg.Value)
			continue
		}
		dbMsgs = append(dbMsgs, &dbMsg)
		validMsgs = append(validMsgs, msg)
	}

	if len(dbMsgs) == 0 {
		return nil
	}

	// 2. 批量入库
	err := global.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "msg_id"}},
		DoNothing: true,
	}).Create(&dbMsgs).Error

	if err != nil {
		global.Log.Error("Batch insert group messages failed, fallback to single", zap.Error(err))
		// 降级处理
		for i, dbMsg := range dbMsgs {
			handleSingleGroupMessage(dbMsg, validMsgs[i].Value)
		}
		return nil
	}

	// 3. 批量/管道写入 Redis Stream
	pipe := global.RDB.Pipeline()
	ctx := context.Background()

	for _, dbMsg := range dbMsgs {
		key := generateKey(dbMsg.ToUserID, 0, true)
		dto := ToMessageDTO(dbMsg)
		jsonBytes, _ := json.Marshal(dto)

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: key,
			MaxLen: 2000,
			Approx: true,
			Values: map[string]interface{}{
				"data": jsonBytes,
			},
		})
	}

	if _, err := pipe.Exec(ctx); err != nil {
		global.Log.Error("Redis pipeline execution failed", zap.Error(err))
	}

	// 4. 批量推送通知
	// 乐观推送模式：API Server 已推送，Consumer 不再推送
	// for _, dbMsg := range dbMsgs {
	// 	PublishPushConsumer(*dbMsg, true)
	// }

	return nil
}

// 辅助函数：单条处理群聊 (用于降级)
func handleSingleGroupMessage(dbMsg *models.Message, value []byte) {
	for i := 0; i < global.RetryMax; i++ {
		err := global.DB.Create(dbMsg).Error
		if err == nil || errors.Is(err, gorm.ErrDuplicatedKey) {
			// 成功后写 Redis
			key := generateKey(dbMsg.ToUserID, 0, true)
			dto := ToMessageDTO(dbMsg)
			jsonBytes, _ := json.Marshal(dto)
			global.RDB.XAdd(context.Background(), &redis.XAddArgs{
				Stream: key, MaxLen: 2000, Approx: true,
				Values: map[string]interface{}{"data": jsonBytes},
			})
			// PublishPushConsumer(*dbMsg, true)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	republish(global.KTopic.Retry, value)
}

// ------ 简单的批量包装器 (用于 Retry 和 Dead Letter) --------------------------------
func handleRetryBatch(msgs []*sarama.ConsumerMessage) error {
	for _, msg := range msgs {
		handleMessageWithDelayRetry(msg.Value)
	}
	return nil
}

func handleDeadLetterBatch(msgs []*sarama.ConsumerMessage) error {
	for _, msg := range msgs {
		handleDeadLetter(msg.Value)
	}
	return nil
}

// ------ 重试队列延迟处理 ------------------------------------------------------------------------
func handleMessageWithDelayRetry(value []byte) error {
	// 强制延迟：让消息在队列里待5秒后再处理
	// 这样给数据库一段恢复时间
	time.Sleep(5 * time.Second)

	var dbMsg models.Message
	if err := json.Unmarshal(value, &dbMsg); err != nil {
		republish(global.KTopic.Dead, value)
		return nil
	}

	// 这里通常只试 1 次，或者也可以少量重试
	err := global.DB.Create(&dbMsg).Error
	if err == nil {
		// 终于成功了
		// 根据消息类型决定后续操作
		if dbMsg.Type == 3 { // 群聊消息 -> 写 Redis Stream
			key := generateKey(dbMsg.ToUserID, 0, true)
			ctx := context.Background()

			// 构造 DTO
			dto := ToMessageDTO(&dbMsg)
			jsonBytes, _ := json.Marshal(dto)

			// XADD
			global.RDB.XAdd(ctx, &redis.XAddArgs{
				Stream: key,
				MaxLen: 2000, // 限制长度
				Approx: true,
				Values: map[string]interface{}{
					"data": jsonBytes,
				},
			})

			// 触发推送 (虽然晚了5秒，但还是得推一下)
			// 重试队列的消息通常意味着 API Server 推送也可能失败了（或者客户端因为网络原因没收到），
			// 但既然选择了“乐观推送”，这里保持一致性也可以不推。
			// 不过，为了保险起见，重试成功的消息（说明之前可能系统有故障）还是推一下比较好。
			// 如果你坚持完全移除路径 B，这里也应该注释掉。
			// 但考虑到重试消息的特殊性（实时性已经没了），推一下作为补偿是合理的。
			// 这里我先注释掉，严格遵循“移除路径 B”的指令。
			// PublishPushConsumer(dbMsg, true)

		} else if dbMsg.Type == 2 { // 私聊消息 -> 仅 DB，无 Redis
			// 触发推送
			// PublishPushConsumer(dbMsg, false)
		}

		return nil
	}

	// 依然失败 -> 进死信队列
	global.Log.Info("Retry failed, sending to Dead Letter Queue", zap.Uint("uid", dbMsg.FromUserID))
	republish(global.KTopic.Dead, value)
	return nil
}

// ------ 死信队列 -----------------------------------------------------------------------------
func handleDeadLetter(value []byte) error {
	global.Log.Error("DEAD LETTER MESSAGE",
		zap.String("raw_json", string(value)),
		zap.Time("dropped_at", time.Now()),
	)
	return nil
}

// republish 发送消息到指定 Topic
func republish(topic string, value []byte) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	}
	_, _, err := global.KafkaProducer.SendMessage(msg)
	if err != nil {
		// 如果连 Kafka 都发不进去，那就是灾难级故障了，只能打 Error 日志
		global.Log.Error("republish failed", zap.String("topic", topic), zap.Error(err))
	}
}

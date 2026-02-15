package service

import (
	"context"
	"encoding/json"
	"go-chat/global"
	"go-chat/internal/models"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
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
		global.KTopic.GroupMsg,
		global.KTopic.Retry,
		global.KTopic.Dead,
	}

	// 定义 Handler
	handler := &ConsumerGroupHandler{
		Handlers: map[string]func([]byte) error{
			global.KTopic.GroupMsg: handleGroupMessage,
			global.KTopic.Retry:    handleMessageWithDelayRetry,
			global.KTopic.Dead:     handleDeadLetter,
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

	global.Log.Info("Consumer Group started", zap.Strings("topics", topics), zap.String("group", groupID))
}

// ConsumerGroupHandler 实现 sarama.ConsumerGroupHandler 接口
type ConsumerGroupHandler struct {
	Handlers map[string]func([]byte) error
}

func (h *ConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *ConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *ConsumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	handler, ok := h.Handlers[claim.Topic()]
	if !ok {
		return nil
	}

	for msg := range claim.Messages() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					global.Log.Error("process message panic", zap.Any("err", r))
				}
			}()

			// 处理消息
			if err := handler(msg.Value); err == nil {
				// 只有无错误才标记为已消费
				sess.MarkMessage(msg, "")
			}
		}()
	}
	return nil
}

// ------ handleGroupMessage 处理群聊消息 --------------------------------------------------------
func handleGroupMessage(value []byte) error {
	var dbMsg models.Message

	// 反序列化失败，脏数据直接投死信队列
	if err := json.Unmarshal(value, &dbMsg); err != nil {
		republish(global.KTopic.Dead, value)
		return nil // 返回 nil 以便 MarkMessage
	}

	// 本地重试
	for i := 0; i < global.RetryMax; i++ {
		err := global.DB.Create(&dbMsg).Error
		if err == nil {
			// 成功！
			// 1. 将新消息追加到 Redis List 尾部，并维持长度 <= 2000
			key := generateKey(dbMsg.ToUserID, 0, true)
			ctx := context.Background()

			// 构造 DTO
			dto := ToMessageDTO(&dbMsg)
			jsonBytes, _ := json.Marshal(dto)

			// 使用 Pipeline 执行 RPush + LTrim
			pipe := global.RDB.Pipeline()
			pipe.RPush(ctx, key, jsonBytes)
			pipe.LTrim(ctx, key, -2000, -1)       // 保留最后 2000 条
			pipe.Expire(ctx, key, 7*24*time.Hour) // 刷新过期时间
			pipe.Exec(ctx)

			// 2. 推送给群成员 (通过 Redis Pub/Sub 通知 Server)
			PublishPushConsumer(dbMsg, true)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 重试耗尽 -> 降级到 Retry Topic
	global.Log.Warn("Group message retry failed, sending to Retry Topic", zap.Any("msg", dbMsg))
	republish(global.KTopic.Retry, value)
	return nil
}

// ------ 重试队列延迟处理 ------------------------------------------------------------------------
func handleMessageWithDelayRetry(value []byte) error {
	// 强制延迟：让消息在队列里待5秒后再处理
	// 这样给数据库一段恢复时间
	time.Sleep(5 * time.Second)

	var dbMsg models.Message
	json.Unmarshal(value, &dbMsg)

	// 这里通常只试 1 次，或者也可以少量重试
	err := global.DB.Create(&dbMsg).Error
	if err == nil {
		// 终于成功了
		key := generateKey(dbMsg.ToUserID, dbMsg.FromUserID, true)
		ctx := context.Background()

		// 构造 DTO
		dto := ToMessageDTO(&dbMsg)
		jsonBytes, _ := json.Marshal(dto)

		// 使用 Pipeline 执行 RPush + LTrim
		pipe := global.RDB.Pipeline()
		pipe.RPush(ctx, key, jsonBytes)
		pipe.LTrim(ctx, key, -2000, -1) // 保留最后 2000 条
		pipe.Expire(ctx, key, 7*24*time.Hour)
		pipe.Exec(ctx)

		PublishPushConsumer(dbMsg, false)
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

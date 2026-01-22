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
	consumer, _ := sarama.NewConsumer([]string{"localhost:9092"}, nil)

	// 启动主消费者
	go consumeLoop(consumer, global.KTopic.ChatMsg, handleMessageWithLocalRetry)

	// 启动重试消费者
	go consumeLoop(consumer, global.KTopic.Retry, handleMessageWithDelayRetry)

	// 启动死信消费者
	go consumeLoop(consumer, global.KTopic.Dead, handleDeadLetter)

}

// 消费循环骨架
func consumeLoop(consumer sarama.Consumer, topic string, handler func([]byte) error) {
	partitionList, _ := consumer.Partitions(topic)
	for partition := range partitionList {
		pc, _ := consumer.ConsumePartition(topic, int32(partition), sarama.OffsetNewest)
		go func(pc sarama.PartitionConsumer) {
			defer pc.Close()
			for msg := range pc.Messages() {
				// 调用具体的处理逻辑
				handler(msg.Value)
			}
		}(pc)
	}
}

// 核心业务处理 + 本地重试
func handleMessageWithLocalRetry(value []byte) error {
	var dbMsg models.Message
	if err := json.Unmarshal(value, &dbMsg); err != nil {
		// 格式错误直接进死信，因为重试也没用
		republish(global.KTopic.Dead, value)
		return nil
	}

	// 本地重试retrymax次
	for i := 0; i < global.RetryMax; i++ {
		err := global.DB.Create(&dbMsg).Error
		if err == nil {
			// 成功！后续推送逻辑...
			key := generateKey(dbMsg.ToUserID, dbMsg.FromUserID)
			global.RDB.Del(context.Background(), key)
			PushMessageToUser(dbMsg)
			return nil
		}
		time.Sleep(100 * time.Millisecond) // 短暂避让
	}

	// 本地重试耗尽 -> 降级到 Retry Topic
	global.Log.Warn("Local retry failed, sending to Retry Topic", zap.Any("msg", dbMsg))
	republish(global.KTopic.Retry, value)
	return nil
}

// 重试队列延迟处理
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
		key := generateKey(dbMsg.ToUserID, dbMsg.FromUserID)
		global.RDB.Del(context.Background(), key)
		PushMessageToUser(dbMsg)
		return nil
	}

	// 依然失败 -> 进死信队列
	global.Log.Info("Retry failed, sending to Dead Letter Queue", zap.Uint("uid", dbMsg.FromUserID))
	republish(global.KTopic.Dead, value)
	return nil
}

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

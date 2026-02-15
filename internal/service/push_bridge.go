package service

import (
	"context"
	"encoding/json"
	"go-chat/global"
	"go-chat/internal/models"

	"go.uber.org/zap"
)

const (
	PushChannel       = "chat:push"
	PushTypeUser      = 1
	PushTypeGroup     = 2
	PushTypeBroadcast = 3
)

type PushPayload struct {
	Type    int            `json:"type"`
	Message models.Message `json:"message"`
}

// PublishPushConsumer 在 Consumer 端调用，将消息推送到 Redis
func PublishPushConsumer(msg models.Message, isGroup bool) {
	payload := PushPayload{
		Message: msg,
	}
	if isGroup {
		payload.Type = PushTypeGroup
	} else {
		payload.Type = PushTypeUser
	}

	data, err := json.Marshal(payload)
	if err != nil {
		global.Log.Error("marshal push payload failed", zap.Error(err))
		return
	}

	if err := global.RDB.Publish(context.Background(), PushChannel, data).Err(); err != nil {
		global.Log.Error("publish push message failed", zap.Error(err))
		return
	}

}

// StartPushListener 在 Server 端调用，监听 Redis 并推送给 WebSocket
func StartPushListener() {
	go func() {
		global.Log.Info("Start Redis Push Listener", zap.String("channel", PushChannel))
		pubsub := global.RDB.Subscribe(context.Background(), PushChannel)
		defer pubsub.Close()

		ch := pubsub.Channel()

		for msg := range ch {
			var payload PushPayload
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				global.Log.Error("unmarshal push payload failed", zap.Error(err))
				continue
			}

			switch payload.Type {
			case PushTypeUser:
				PushMessageToUser(payload.Message)
			case PushTypeGroup:
				PushMessageToGroup(payload.Message)
			}
		}
	}()
}

package global

import (
	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	Config        *viper.Viper
	DB            *gorm.DB
	Log           *zap.Logger
	RDB           *redis.Client
	KafkaProducer sarama.SyncProducer
)

type KafkaTopic struct {
	ChatMsg string
	Retry   string
	Dead    string
}

var KAdrrs []string

var KTopic KafkaTopic

const RetryMax = 3

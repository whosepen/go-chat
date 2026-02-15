package global

import (
	pb "go-chat/proto/oss"

	"github.com/IBM/sarama"
	"github.com/minio/minio-go/v7"
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
	MinioClient   *minio.Client
)

var MinioImgHost string

type KafkaTopic struct {
	ChatMsg  string
	GroupMsg string
	Retry    string
	Dead     string
}

var KAdrrs []string

var KTopic KafkaTopic

const RetryMax = 3

var OssClient pb.OssServiceClient

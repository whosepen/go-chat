package initial

import (
	"fmt"
	"go-chat/global"

	"github.com/IBM/sarama"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func InitKafka() {

	global.KAdrrs = viper.GetStringSlice("kafka.addr")
	if len(global.KAdrrs) == 0 {
		global.KAdrrs = []string{"localhost:9092"}
	}

	config := sarama.NewConfig()

	ack := viper.GetString("kafka.ack")
	switch ack {
	case "all":
		config.Producer.RequiredAcks = sarama.WaitForAll
	case "1":
		config.Producer.RequiredAcks = sarama.WaitForLocal
	default:
		config.Producer.RequiredAcks = sarama.NoResponse
	}

	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.Return.Successes = true

	// 配置重试次数
	config.Producer.Retry.Max = viper.GetInt("kafka.retry")

	// 连接 Kafka
	producer, err := sarama.NewSyncProducer(global.KAdrrs, config)
	if err != nil {
		global.Log.Fatal("Kafka connect failed", zap.Error(err))
	}

	global.KafkaProducer = producer
	global.Log.Info("Kafka connected successfully")

	// 使用 Admin 接口自动创建 Topic
	admin, err := sarama.NewClusterAdmin(global.KAdrrs, config)
	if err != nil {
		global.Log.Error("Kafka admin create failed", zap.Error(err))
		return
	}
	defer admin.Close()

	global.KTopic.ChatMsg = viper.GetString("kafka.topic.chat")
	NewTopic(admin, global.KTopic.ChatMsg, 1, 1)

	global.KTopic.Retry = viper.GetString("kafka.topic.retry")
	NewTopic(admin, global.KTopic.Retry, 1, 1)

	global.KTopic.Dead = viper.GetString("kafka.topic.dead")
	NewTopic(admin, global.KTopic.Dead, 1, 1)

}

func NewTopic(admin sarama.ClusterAdmin, topicName string, numPartitions int32, replicationFactor int16) {
	topics, _ := admin.ListTopics()
	if _, ok := topics[topicName]; !ok {
		err := admin.CreateTopic(topicName, &sarama.TopicDetail{
			NumPartitions:     numPartitions,     // 分区数
			ReplicationFactor: replicationFactor, // 副本数 (单机部署必须是1)
		}, false)
		if err != nil {
			global.Log.Fatal(fmt.Sprintf("Create topic %s failed", topicName), zap.Error(err))
		} else {
			global.Log.Info(fmt.Sprintf("Topic %s created successfully", topicName))
		}
	}
}

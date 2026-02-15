package initial

import (
	"context"
	"fmt"
	"go-chat/global"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func InitMinio() {
	endpoint := viper.GetString("minio.endpoint") // MinIO 服务地址
	if endpoint == "" {
		global.Log.Fatal("no endpoint in config")
	}
	accessKeyID := viper.GetString("minio.accessKeyID")
	secretAccessKey := viper.GetString("secretAccessKey")
	useSSL := viper.GetBool("minio.useSSL") // config为空默认为false
	imgHost := viper.GetString("minio.imgHost")

	// 初始化客户端
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		global.Log.Fatal("minio客户端初始化失败", zap.Error(err))
	}

	global.MinioImgHost = imgHost
	global.MinioClient = minioClient
	global.Log.Info("MinIO connected successfully")

	// 检查 Bucket 是否存在
	buckets := viper.GetStringSlice("minio.buckets")
	for _, bucketName := range buckets {
		_, err = minioClient.BucketExists(context.Background(), bucketName)
		if err != nil {
			global.Log.Fatal(fmt.Sprintf("necessary bucket [%s] doesnt exist", bucketName))
			return
		}
	}

}

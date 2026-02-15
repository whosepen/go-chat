package initial

import (
	"fmt"
	"go-chat/global"
	pb "go-chat/proto/oss"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitOssClient() {
	addr := viper.GetString("oss.addr")
	if addr == "" {
		addr = "localhost:50051"
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		global.Log.Error("did not connect to oss service", zap.Error(err))
		return
	}
	// 注意：这里没有 defer conn.Close()，因为我们希望连接在应用程序生命周期内保持打开
	// 如果需要优雅关闭，可以将其存储在 global 中并在 shutdown 时关闭

	global.OssClient = pb.NewOssServiceClient(conn)
	global.Log.Info(fmt.Sprintf("OSS Client connected to %s", addr))
}

package cmd

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"go-chat/internal/oss"
	"go-chat/internal/pkg/initial"
	pb "go-chat/proto/oss"
)

var ossCmd = &cobra.Command{
	Use:   "oss",
	Short: "Start OSS Microservice",
	Run: func(cmd *cobra.Command, args []string) {
		initial.InitLogger()
		// 初始化 MinIO
		initial.InitMinio()
		fmt.Println("MinIO Initialized.")

		// 2. 启动 gRPC 监听
		lis, err := net.Listen("tcp", ":50051") // 监听 50051 端口
		if err != nil {
			panic(err)
		}

		// 3. 注册服务
		grpcServer := grpc.NewServer()
		pb.RegisterOssServiceServer(grpcServer, &oss.OssGrpcServer{})

		fmt.Println("OSS gRPC Service is running on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(ossCmd)
}

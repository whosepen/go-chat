package oss

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"go-chat/global"
	pb "go-chat/proto/oss" // 生成的 proto 代码
)

// OssGrpcServer 实现 proto 定义的接口
type OssGrpcServer struct {
	pb.UnimplementedOssServiceServer
}

func (s *OssGrpcServer) GetUploadCredential(ctx context.Context, req *pb.GetUploadCredentialRequest) (*pb.GetUploadCredentialResponse, error) {
	// 1. 业务分桶逻辑
	var bucketName string
	switch req.FileType {
	case "avatar":
		bucketName = "user-avatars"
	case "chat":
		bucketName = "chat-files"
	default:
		bucketName = "temp-files"
	}

	// 2. 生成唯一文件名
	ext := filepath.Ext(req.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	objectName := fmt.Sprintf("%s/%s%s", time.Now().Format("20060102"), uuid.New().String(), ext)

	// 3. 生成上传凭证 (核心)
	expiry := time.Minute * 10
	presignedURL, err := global.MinioClient.PresignedPutObject(ctx, bucketName, objectName, expiry)
	if err != nil {
		return nil, err
	}

	// 4. 拼接访问链接 (注意：这里要返回公网可访问的地址)
	publicUrl := fmt.Sprintf("%s/%s/%s", global.MinioImgHost, bucketName, objectName)

	return &pb.GetUploadCredentialResponse{
		UploadUrl: presignedURL.String(),
		PublicUrl: publicUrl,
		Key:       objectName,
	}, nil
}

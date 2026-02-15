package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"go-chat/global"
	"go-chat/internal/pkg/utils"
	pb "go-chat/proto/oss"
)

// GetUploadToken
func GetUploadToken(c *gin.Context) {
	filename := c.GetString("filename")
	fileType := c.GetString("type") // avatar, chat

	userID := c.GetUint("userID")
	if userID == 0 {
		utils.Fail(c, "无效上传用户")
		return
	}

	// 调用远程 OSS 服务
	// 设置 3 秒超时，防止 OSS 挂了卡死主服务
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &pb.GetUploadCredentialRequest{
		Filename: filename,
		FileType: fileType,
		UserId:   int64(userID),
	}

	if global.OssClient == nil {
		utils.ServerError(c, "OSS service unavailable")
		return
	}

	resp, err := global.OssClient.GetUploadCredential(ctx, req)
	if err != nil {
		utils.ServerError(c, "Failed to get upload token")
		return
	}

	// [成功] 拿到 URL，返回给前端
	// 此时文件还没上传，这里只是把“门票”给前端
	utils.Success(c, gin.H{
		"put_url":  resp.UploadUrl, // 前端用 PUT 上传
		"file_url": resp.PublicUrl, // 上传成功后用于显示的 URL
		"key":      resp.Key,       // 文件的 Key
	})
}

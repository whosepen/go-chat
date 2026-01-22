package service

import (
	"fmt"
	"go-chat/global"
	"go-chat/internal/models"

	"go.uber.org/zap"
)

func CreateRightNow(data models.ModelInterface) error { // data要求必须传结构体指针
	if err := global.DB.Create(data).Error; err != nil {
		global.Log.Error(fmt.Sprintf("save %s failed", data.TableName()), zap.Error(err))
		return err
	}
	return nil
}

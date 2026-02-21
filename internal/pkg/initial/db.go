package initial

import (
	"database/sql"
	"fmt"
	"go-chat/global"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB() {
	dsn := viper.GetString("mysql.dsn")
	if dsn == "" {
		panic("mysql dsn is empty")
	}

	newLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	})

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: newLogger})
	if err != nil {
		// Try to handle "Unknown database" error
		if strings.Contains(err.Error(), "1049") {
			global.Log.Info("Database does not exist, attempting to create it...")
			if createErr := createDatabase(dsn); createErr != nil {
				global.Log.Fatal("failed to create database", zap.Error(createErr))
			}
			// Retry connection
			db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: newLogger})
			if err != nil {
				global.Log.Fatal("connect mysql failed after creating db", zap.Error(err))
			}
		} else {
			global.Log.Fatal("connect mysql failed", zap.Error(err))
		}
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	global.DB = db
	global.Log.Info("mysql connected successfully")
}

func createDatabase(dsn string) error {
	idxQuestion := strings.Index(dsn, "?")
	var dsnWithoutParams, params string
	if idxQuestion != -1 {
		dsnWithoutParams = dsn[:idxQuestion]
		params = dsn[idxQuestion:]
	} else {
		dsnWithoutParams = dsn
		params = ""
	}

	idxSlash := strings.LastIndex(dsnWithoutParams, "/")
	if idxSlash == -1 {
		return fmt.Errorf("invalid dsn format: no database name separator found")
	}

	dbName := dsnWithoutParams[idxSlash+1:]
	baseDSN := dsnWithoutParams[:idxSlash+1] + params

	db, err := sql.Open("mysql", baseDSN)
	if err != nil {
		return fmt.Errorf("failed to open base connection: %w", err)
	}
	defer db.Close()

	createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", dbName)
	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to execute create database: %w", err)
	}

	global.Log.Info("Database created successfully", zap.String("db", dbName))
	return nil
}

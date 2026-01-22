package initial

import (
	"go-chat/global"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger 初始化日志组件
// 包含功能：文件持久化、自动切割、控制台双写
func InitLogger() {
	// 1. 获取配置的日志级别，默认为 Debug
	// 生产环境建议调整为 Info 或 Warn
	logLevel := zapcore.DebugLevel

	// 2. 配置 Core
	// 我们使用 NewTee 将日志分发到两个地方：文件 和 控制台
	core := zapcore.NewTee(
		getFileCore(logLevel),    // 文件输出 Core
		getConsoleCore(logLevel), // 控制台输出 Core
	)

	// 3. 创建 Logger
	// AddCaller: 在日志中显示是哪个文件哪行代码打印的 (例如: service/consumer.go:45)
	logger := zap.New(core, zap.AddCaller())

	// 4. 替换全局变量
	global.Log = logger
}

// getFileCore 构建文件输出的核心配置
func getFileCore(level zapcore.Level) zapcore.Core {
	// 设置 Lumberjack 日志切割
	writeSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "./logs/app.log", // 日志文件路径
		MaxSize:    10,               // 单个文件最大尺寸 (MB)
		MaxBackups: 10,               // 最多保留备份个数
		MaxAge:     30,               // 最多保留天数
		Compress:   true,             // 是否压缩备份 (gzip)，节省磁盘空间
	})

	// 文件日志使用 JSON 格式，方便 ELK 等工具采集
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	return zapcore.NewCore(encoder, writeSyncer, level)
}

// getConsoleCore 构建控制台输出的核心配置
func getConsoleCore(level zapcore.Level) zapcore.Core {
	// 控制台直接打印到 Stdout
	consoleSyncer := zapcore.AddSync(os.Stdout)

	// 控制台使用 Console 格式 (非JSON)，且开启颜色，方便开发者阅读
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05") // 控制台时间简短点
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder       // 开启颜色
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	return zapcore.NewCore(encoder, consoleSyncer, level)
}

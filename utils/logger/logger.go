package logger

import (
	"path"

	"github.com/farnese17/chat/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

func SetupLogger() *zap.Logger {
	logPath := config.GetConfig().Common().LogDir()
	logPath = path.Clean(logPath)

	var filename string
	if path.Ext(logPath) != "" {
		filename = logPath
	} else {
		filename = path.Join(logPath, "/chat.log")
	}

	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filename,
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     28,
		LocalTime:  true,
	})

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		w,
		zap.DebugLevel,
	)

	logger = zap.New(core, zap.AddCaller())
	logger.Info("Logger with log rotation initialized")
	return logger
}

func GetLogger() *zap.Logger {
	return logger
}

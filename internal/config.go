package internal

import (
	"os"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() *zap.Logger {
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "prod"
	}

	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/app.log",
		MaxSize:    1, // megabytes
		MaxBackups: 30,
		MaxAge:     30, // days
	})

	c := zapcore.EncoderConfig{
		MessageKey:     "message",
		LevelKey:       "level",
		TimeKey:        "time",
		NameKey:        "logger",
		CallerKey:      "file",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	encoder := zapcore.NewJSONEncoder(c)
	stackTraceLevel := zap.AddStacktrace(zapcore.WarnLevel)
	if environment == "dev" {
		encoder = zapcore.NewConsoleEncoder(c)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), w),
		zap.InfoLevel,
	)

	logger := zap.New(core, zap.AddCaller(), stackTraceLevel)

	return logger
}

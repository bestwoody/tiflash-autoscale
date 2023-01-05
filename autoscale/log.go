package autoscale

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var RawLogger *zap.Logger
var Logger *zap.SugaredLogger

func InitZapLogger() {
	RawLogger, err := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()
	if err != nil {
		panic(err)
	}
	Logger = RawLogger.Sugar()
}

func Loginfo(format string, v ...any) {
	Logger.Infof(format, v...)
}

func Logwarn(format string, v ...any) {
	Logger.Warnf(format, v...)
}

func Logerr(format string, v ...any) {
	Logger.Errorf(format, v...)
}

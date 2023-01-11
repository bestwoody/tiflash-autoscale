package autoscale

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var RawLogger *zap.Logger
var Logger *zap.SugaredLogger

const SettingLogLevel = zap.DebugLevel

func InitZapLogger() {
	if Logger != nil {
		return
	}
	customTimeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + t.Format("2006-01-02 15:04:05.000Z07:00") + "]")
	}

	customLevelEncoder := func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + level.CapitalString() + "]")
	}

	customCallerEncoder := func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + caller.TrimmedPath() + "]")
	}

	RawLogger, err := zap.Config{
		Level:       zap.NewAtomicLevelAt(SettingLogLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:          "ts",
			LevelKey:         "level",
			NameKey:          "logger",
			CallerKey:        "caller",
			FunctionKey:      zapcore.OmitKey,
			MessageKey:       "msg",
			StacktraceKey:    "stacktrace",
			ConsoleSeparator: " ",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      customLevelEncoder,
			EncodeTime:       customTimeEncoder,
			EncodeDuration:   zapcore.SecondsDurationEncoder,
			EncodeCaller:     customCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()
	if err != nil {
		panic(err)
	}
	Logger = RawLogger.Sugar()
}

func init() {
	InitZapLogger()
}

// func Logger.Infof(format string, v ...any) {
// 	Logger.Infof(format, v...)
// }

// func Logger.Warnf(format string, v ...any) {
// 	Logger.Warnf(format, v...)
// }

// func Logger.Errorf(format string, v ...any) {
// 	Logger.Errorf(format, v...)
// }

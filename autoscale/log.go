package autoscale

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var RawLogger *zap.Logger
var Logger *zap.SugaredLogger

const SettingLogLevel = zap.DebugLevel

const (
	LogModePod = iota
	LogModeLocalTest
)

var LogMode = LogModePod

func getLogPath() string {
	// if LogMode == LogModePod {
	return "/var/log/"
	// } else if LogMode == LogModeLocalTest {
	// return "./"
	// } else {
	// panic(fmt.Errorf("unknown LogMode:%v", LogMode))
	// }
}

func openSinks() (zapcore.WriteSyncer, zapcore.WriteSyncer, error) {
	sink, closeOut, err := zap.Open("stderr")
	if err != nil {
		return nil, nil, err
	}
	errSink, _, err := zap.Open("stderr")
	if err != nil {
		closeOut()
		return nil, nil, err
	}
	return sink, errSink, nil
}

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
	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   getLogPath() + "autoscale.log",
		MaxSize:    500, // megabytes
		MaxBackups: 10,
		MaxAge:     28, // days
	})
	// encoding := "console"
	encoderConfig := zapcore.EncoderConfig{
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
	}
	errorOutputPaths := []string{"stderr"}
	if LogMode != LogModeLocalTest {
		errorOutputPaths = []string{"stderr", getLogPath() + "autoscale_err.log"}
	}
	RawLogger, err := zap.Config{
		Level:       zap.NewAtomicLevelAt(SettingLogLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: errorOutputPaths,
	}.Build()

	// sink, errSink, err := openSinks()
	if err != nil {
		panic(err)
	}
	if LogMode != LogModeLocalTest {
		core := zapcore.NewTee(RawLogger.Core(), zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			w,
			SettingLogLevel,
		))
		RawLogger = zap.New(core, zap.AddCaller())
	}
	// RawLogger = zap.New()

	Logger = RawLogger.Sugar()
}

// func init() {
// 	InitZapLogger()
// }

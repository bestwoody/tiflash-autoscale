package main

import (
	"time"

	"github.com/tikv/pd/autoscale"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type TestIntf interface {
	HeheFunc()
}

type Hehe struct {
	val int
}

func (c *Hehe) HeheFunc() {
	c.val = 2
}

func testFunc(ti TestIntf) {
	ti.HeheFunc()
}

type MockTenantInfoProvider struct {
	tsContainer *autoscale.TimeSeriesContainer
	podMap      map[string]int
}

func (c *MockTenantInfoProvider) GetTenantInfoOfPod(podName string) (string, int64) {
	v, ok := c.podMap[podName]
	if !ok {
		c.podMap[podName] = 1
	} else {
		v++
	}
	return "t1", time.Now().Unix() - 1800
}
func (c *MockTenantInfoProvider) GetTenantScaleIntervalSec(tenant string) (int, bool) {
	return 300, false
}

func (c *MockTenantInfoProvider) InsertWithUserCfg(key string, time int64, values []float64, cfgIntervalSec int) bool /* is_success */ {
	return c.tsContainer.InsertWithUserCfg(key, time, values, cfgIntervalSec)
	// Logger.Infof("insert %v %v %v %v", key, time, values, cfgIntervalSec)
	// return true
}

func main2() {

	mtip := MockTenantInfoProvider{
		podMap: make(map[string]int),
	}
	// // RangeQuery(5*time.Minute, 10*time.Second)
	cli, err := autoscale.NewPromClient("http://localhost:16292")

	tsContainer := autoscale.NewTimeSeriesContainer(cli)
	mtip.tsContainer = tsContainer
	if err != nil {
		panic(err)
	}
	cli.RangeQueryCpu(time.Hour, 10*time.Second, &mtip, &mtip)
	for k, v := range mtip.podMap {
		Logger.Infof("pod:%v call_cnt:%v", k, v)
		mtip.tsContainer.Dump(k, autoscale.MetricsTopicCpu)
	}
}

func NewZapLogger() (*zap.Logger, error) {
	return zap.Config{
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
}

func zapLogTest() {
	logger, _ := NewZapLogger()
	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()
	sugar.Infow("failed to fetch URL",
		// Structured context as loosely typed key-value pairs.
		"url", "urlval",
		"attempt", 3,
		"backoff", time.Second,
	)
	sugar.Infof("Failed to fetch URL: %s", "hehe")
}
func main() {
	zapLogTest()
	// cli, err := autoscale.NewPromClient("http://localhost:16292")
	// if err != nil {
	// 	panic(err)
	// }
	// cli.QueryComputeTask()
}

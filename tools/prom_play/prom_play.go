package main

import (
	"log"
	"time"

	"github.com/tikv/pd/autoscale"
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
	// log.Printf("insert %v %v %v %v\n", key, time, values, cfgIntervalSec)
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
		log.Printf("pod:%v call_cnt:%v\n", k, v)
		mtip.tsContainer.Dump(k, autoscale.MetricsTopicCpu)
	}
}

func main() {
	cli, err := autoscale.NewPromClient("http://localhost:16292")
	if err != nil {
		panic(err)
	}
	cli.QueryComputeTask()
}

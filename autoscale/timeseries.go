package autoscale

import (
	"container/list"
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func Max(a int64, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}

func Min(a int64, b int64) int64 {
	if a < b {
		return a
	} else {
		return b
	}
}

func MaxInt(a int, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func MinInt(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

type TimeValues struct {
	time   int64
	values []float64
	window time.Duration
}

type AvgSigma struct {
	sum float64
	cnt int64
}

type SimpleTimeSeries struct {
	series     *list.List // elem type: TimeValues
	Statistics []AvgSigma
	// min_time   int64
	max_time int64
	cap      int // cap = [tenant's scale_interval] / step
}

func (c *SimpleTimeSeries) Reset() {
	for c.series.Len() > 0 {
		c.series.Remove(c.series.Front())
	}
	for i := range c.Statistics {
		c.Statistics[i].Reset()
	}
	c.max_time = 0
}

type TimeValPair struct {
	time  int64
	value float64
}

func (c *SimpleTimeSeries) Dump(podName string) {
	l := c.series
	arr := make([]TimeValPair, 0, l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		ts := e.Value.(*TimeValues)
		if len(ts.values) > 0 {
			arr = append(arr, TimeValPair{ts.time, ts.values[0]})
		} else {
			arr = append(arr, TimeValPair{ts.time, -1})
		}
		// do something with e.Value
	}
	log.Printf("[SimpleTimeSeries]podname: %v , dump arr: %v %+v\n", podName, len(arr), arr)
}

func (c *SimpleTimeSeries) Cpu() *AvgSigma {
	return &c.Statistics[0]
}

func (c *SimpleTimeSeries) Mem() *AvgSigma {
	return &c.Statistics[1]
}

func (cur *AvgSigma) Reset() {
	cur.cnt = 0
	cur.sum = 0
}

func (cur *AvgSigma) Sub(v float64) {
	cur.cnt--
	cur.sum -= v
}

func (cur *AvgSigma) Add(v float64) {
	cur.cnt++
	cur.sum += v
}

func (cur *AvgSigma) Avg() float64 {
	if cur.cnt <= 0 {
		return 0
	}
	return cur.sum / float64(cur.cnt)
}

func (cur *AvgSigma) Cnt() int64 {
	return cur.cnt
}

func (cur *AvgSigma) Merge(o *AvgSigma) {
	cur.cnt += o.cnt
	cur.sum += o.sum
}

func Sub(cur []AvgSigma, values []float64) {
	if len(values) == 0 {
		log.Printf("[error]Sub error empty values\n")
	}
	for i, value := range values {
		cur[i].Sub(value)
	}
}

func Add(cur []AvgSigma, values []float64) {
	for i, value := range values {
		cur[i].Add(value)
	}
}

func Merge(cur []AvgSigma, o []AvgSigma) {
	if o == nil {
		return
	}
	for i, value := range o {
		cur[i].Merge(&value)
	}
}

func Avg(cur []AvgSigma) []float64 {
	ret := make([]float64, 3)
	for _, elem := range cur {
		ret = append(ret, elem.Avg())
	}
	return ret
}

type StatsOfTimeSeries struct {
	AvgOfCpu       float64
	AvgOfMem       float64
	SampleCntOfCpu int64
	SampleCntOfMem int64
	MinTime        int64
	MaxTime        int64
}

type TimeSeriesContainer struct {
	seriesMap          map[string]*SimpleTimeSeries
	defaultCapOfSeries int
	mu                 sync.Mutex
}

func NewTimeSeriesContainer(defaultCapOfSeries int) *TimeSeriesContainer {
	return &TimeSeriesContainer{
		seriesMap:          make(map[string]*SimpleTimeSeries),
		defaultCapOfSeries: defaultCapOfSeries}
}

func (c *TimeSeriesContainer) GetStatisticsOfPod(podname string) []AvgSigma {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.seriesMap[podname]
	if !ok {
		return nil
	}
	ret := make([]AvgSigma, CapacityOfStaticsAvgSigma)
	Merge(ret, v.Statistics)
	return ret
}

func (c *TimeSeriesContainer) Dump(podname string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.seriesMap[podname]
	if !ok {
		return
	}
	v.Dump(podname)
}

func (c *TimeSeriesContainer) GetSnapshotOfTimeSeries(podname string) *StatsOfTimeSeries {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.seriesMap[podname]
	if !ok {
		return nil
	}
	minTime, maxTime := v.getMinMaxTime()
	if maxTime == 0 && minTime == 0 {
		return nil
	}
	return &StatsOfTimeSeries{AvgOfCpu: v.Cpu().Avg(),
		SampleCntOfCpu: v.Cpu().Cnt(),
		AvgOfMem:       v.Mem().Avg(),
		SampleCntOfMem: v.Mem().Cnt(),
		MinTime:        minTime, MaxTime: maxTime}
}

func (c *TimeSeriesContainer) ResetMetricsOfPod(podname string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.seriesMap[podname]
	if ok {
		v.Reset()
		log.Printf("[ResetMetricsOfPod]set metrics of pod %v , cnt:%v cond1:%v  cond1&cond2: %v \n", podname, v.Cpu().Cnt(), (v.series != nil), (v.series != nil && v.series.Front() != nil))
	} else {
		log.Printf("[error]Reset pod %v fail\n", podname)
	}
}

func (cur *SimpleTimeSeries) getMinMaxTime() (int64, int64) {
	if cur.series != nil && cur.series.Front() != nil {
		min_time := cur.series.Front().Value.(*TimeValues).time
		return min_time, cur.max_time
	} else {
		log.Printf("[error]getMinMaxTime fail, cnt:%v cond1:%v  cond1&cond2: %v \n", cur.Cpu().Cnt(), (cur.series != nil), (cur.series != nil && cur.series.Front() != nil))
		return 0, 0
	}
}

func (cur *SimpleTimeSeries) append(time int64, values []float64) {
	cur.series.PushBack(
		&TimeValues{
			time:   time,
			values: values,
		})
	if cur.max_time == 0 {
		cur.max_time = time
	} else {
		cur.max_time = Max(cur.max_time, time)
	}
	Add(cur.Statistics, values)
	for cur.series.Len() > cur.cap {
		Sub(cur.Statistics, cur.series.Front().Value.(*TimeValues).values)
		cur.series.Remove(cur.series.Front())
	}
}

func (cur *TimeSeriesContainer) Insert(key string, time int64, values []float64) {
	cur.mu.Lock()
	defer cur.mu.Unlock()
	val, ok := cur.seriesMap[key]
	if !ok {
		val = &SimpleTimeSeries{
			series:     list.New(),
			Statistics: make([]AvgSigma, CapacityOfStaticsAvgSigma),
			cap:        cur.defaultCapOfSeries,
		}
		cur.seriesMap[key] = val
	}
	val.append(time, values)
}

type PromClient struct {
}

func promplay() {
	client, err := api.NewClient(api.Config{
		Address: "http://as-prometheus.tiflash-autoscale.svc.cluster.local:16292",
	})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, warnings, err := v1api.Query(ctx, "up", time.Now(), v1.WithTimeout(5*time.Second))
	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		os.Exit(1)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	fmt.Printf("Result:\n%v\n", result)
}

func RangeQueryCpuTimeSeriesOfPod(scaleIntervalSec int64, metricResolution int, podName string, timeseriesHolder *SimpleTimeSeries) {

}

func RangeQueryCpuTimeSeriesOfAllPods(scaleIntervalSec int64, metricResolution int, tsContainer *TimeSeriesContainer) {

}

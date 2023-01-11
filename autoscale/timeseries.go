package autoscale

import (
	"container/list"
	"context"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
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

// description of SimpleTimeSeries
type DescOfTenantTimeSeries struct {
	MaxOfPodMaxTime        int64
	MinOfPodMaxTime        int64
	MaxOfPodMinTime        int64
	MinOfPodMinTime        int64
	MaxOfPodTimeseriesSize int
	MinOfPodTimeseriesSize int
	SumOfPodTimeseriesSize int
	PodCnt                 int
	// MinIntervalSec         int
	// MaxIntervalSec         int
}

// description of SimpleTimeSeries
type DescOfPodTimeSeries struct {
	MaxTime int64
	MinTime int64
	Size    int
	// IntervalSec int
}

func (c *DescOfTenantTimeSeries) Agg(o *DescOfPodTimeSeries) {
	c.MinOfPodMinTime = Min(c.MinOfPodMinTime, o.MinTime)
	c.MaxOfPodMinTime = Max(c.MaxOfPodMinTime, o.MinTime)
	c.MinOfPodMaxTime = Min(c.MinOfPodMaxTime, o.MaxTime)
	c.MaxOfPodMaxTime = Max(c.MaxOfPodMaxTime, o.MaxTime)
	c.SumOfPodTimeseriesSize += o.Size
	c.MaxOfPodTimeseriesSize = MaxInt(c.MaxOfPodTimeseriesSize, o.Size)
	c.MinOfPodTimeseriesSize = MinInt(c.MinOfPodTimeseriesSize, o.Size)
	c.PodCnt += 1
	// c.MinIntervalSec = MinInt(c.MinIntervalSec, o.IntervalSec)
	// c.MaxIntervalSec = MaxInt(c.MaxIntervalSec, o.IntervalSec)
}

func (c *DescOfTenantTimeSeries) Init(o *DescOfPodTimeSeries) {
	c.MinOfPodMinTime = o.MinTime
	c.MaxOfPodMinTime = o.MinTime
	c.MinOfPodMaxTime = o.MaxTime
	c.MaxOfPodMaxTime = o.MaxTime
	c.SumOfPodTimeseriesSize = o.Size
	c.MaxOfPodTimeseriesSize = o.Size
	c.MinOfPodTimeseriesSize = o.Size
	c.PodCnt = 1
	// c.MinIntervalSec = o.IntervalSec
	// c.MaxIntervalSec = o.IntervalSec
}

type SimpleTimeSeries struct {
	series     *list.List // elem type: TimeValues
	Statistics []AvgSigma
	// min_time   int64
	max_time    int64
	cap         int // cap = [tenant's scale_interval] / step
	intervalSec int
}

func computeSeriesCapBasedOnIntervalSec(newIntervalSec int) int {
	return MaxInt(newIntervalSec/MetricResolutionSeconds, 1)
}

func (c *SimpleTimeSeries) ReloadCfg(newIntervalSec int) {
	c.intervalSec = newIntervalSec
	c.cap = computeSeriesCapBasedOnIntervalSec(newIntervalSec)
}

// func (c *SimpleTimeSeries) Reset() {
// 	for c.series.Len() > 0 {
// 		c.series.Remove(c.series.Front())
// 	}
// 	for i := range c.Statistics {
// 		c.Statistics[i].Reset()
// 	}
// 	c.max_time = 0
// }

type TimeValPair struct {
	time  int64
	value float64
}

func (c *SimpleTimeSeries) Dump(podName string, topic MetricsTopic) {
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
	Logger.Infof("[SimpleTimeSeries]metric_topic:%v podname: %v , dump arr: %v %+v", topic.String(), podName, len(arr), arr)
}

func (c *SimpleTimeSeries) ValsOfMetric() *AvgSigma {
	return &c.Statistics[0]
}

func (c *SimpleTimeSeries) SecondMetricVals() *AvgSigma {
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

func (cur *AvgSigma) Sum() float64 {
	return cur.sum
}

func (cur *AvgSigma) Merge(o *AvgSigma) {
	cur.cnt += o.cnt
	cur.sum += o.sum
}

func Sub(cur []AvgSigma, values []float64) {
	if len(values) == 0 {
		Logger.Errorf("[error]Sub error empty values")
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
	AvgOfVals float64
	// AvgOfMem       float64
	SampleCntOfVals int64
	// SampleCntOfMem int64
	MinTime int64
	MaxTime int64
}

// manage timeseries of all pods
type TimeSeriesContainer struct {
	seriesMap        map[string]*SimpleTimeSeries // cpu metric
	taskCntSeriesMap map[string]*SimpleTimeSeries // task cnt of tiflash metric
	// defaultCapOfSeries int
	mu      sync.Mutex
	promCli *PromClient
}

func NewTimeSeriesContainer(promCli *PromClient) *TimeSeriesContainer {
	return &TimeSeriesContainer{
		seriesMap:        make(map[string]*SimpleTimeSeries),
		taskCntSeriesMap: make(map[string]*SimpleTimeSeries),
		// defaultCapOfSeries: defaultCapOfSeries,
		promCli: promCli,
	}
}

func (c *TimeSeriesContainer) GetStatisticsOfPod(podname string, metricsTopic MetricsTopic) ([]AvgSigma, *DescOfPodTimeSeries) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seriesMap := c.SeriesMap(metricsTopic)
	// if seriesMap == nil {
	// 	return nil
	// }
	v, ok := seriesMap[podname]
	if !ok {
		return nil, nil
	}
	ret := make([]AvgSigma, CapacityOfStaticsAvgSigma)
	Merge(ret, v.Statistics)
	minT, maxT := v.getMinMaxTime()
	stats := &DescOfPodTimeSeries{
		MinTime: minT,
		MaxTime: maxT,
		Size:    v.series.Len(),
		// IntervalSec: v.intervalSec,
	}
	return ret, stats
}

func (c *TimeSeriesContainer) Dump(podname string, topic MetricsTopic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seriesMap := c.SeriesMap(topic)
	// if seriesMap == nil {
	// 	return
	// }
	v, ok := seriesMap[podname]
	if !ok {
		return
	}
	v.Dump(podname, topic)
}

func (c *TimeSeriesContainer) DumpAll(topic MetricsTopic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seriesMap := c.SeriesMap(topic)
	// if seriesMap == nil {
	// 	return
	// }
	for podname, v := range seriesMap {
		v.Dump(podname, topic)
	}
}

func (c *TimeSeriesContainer) GetSnapshotOfTimeSeries(podname string, metricsTopic MetricsTopic) *StatsOfTimeSeries {
	c.mu.Lock()
	defer c.mu.Unlock()
	seriesMap := c.SeriesMap(metricsTopic)
	// if seriesMap == nil {
	// 	return nil
	// }
	v, ok := seriesMap[podname]
	if !ok {
		return nil
	}
	minTime, maxTime := v.getMinMaxTime()
	if maxTime == 0 && minTime == 0 {
		return nil
	}
	return &StatsOfTimeSeries{
		AvgOfVals:       v.ValsOfMetric().Avg(),
		SampleCntOfVals: v.ValsOfMetric().Cnt(),
		// AvgOfMem:       v.Mem().Avg(),
		// SampleCntOfMem: v.Mem().Cnt(),
		MinTime: minTime, MaxTime: maxTime}
}

func (c *TimeSeriesContainer) ResetMetricsOfPod(podname string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.seriesMap[podname]
	if ok {
		delete(c.seriesMap, podname)
		// v.Reset()
		Logger.Infof("[ResetMetricsOfPod]reset cpu metrics of pod %v , cnt:%v cond1:%v  cond1&cond2: %v ", podname, v.ValsOfMetric().Cnt(), (v.series != nil), (v.series != nil && v.series.Front() != nil))
	} else {
		Logger.Warnf("Reset pod %v fail, entry has been removed", podname)
	}

	v, ok = c.taskCntSeriesMap[podname]
	if ok {
		delete(c.seriesMap, podname)
		// v.Reset()
		Logger.Infof("[ResetMetricsOfPod]reset taskcnt metrics of pod %v , cnt:%v cond1:%v  cond1&cond2: %v ", podname, v.ValsOfMetric().Cnt(), (v.series != nil), (v.series != nil && v.series.Front() != nil))
	} else {
		Logger.Warnf("Reset pod %v fail, entry has been removed", podname)
	}
}

func (cur *SimpleTimeSeries) getMinMaxTime() (int64, int64) {
	if cur.series != nil && cur.series.Front() != nil {
		min_time := cur.series.Front().Value.(*TimeValues).time
		return min_time, cur.max_time
	} else {
		Logger.Errorf("[error]getMinMaxTime fail, cnt:%v cond1:%v  cond1&cond2: %v ", cur.ValsOfMetric().Cnt(), (cur.series != nil), (cur.series != nil && cur.series.Front() != nil))
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
	for cur.series.Len() > cur.cap ||
		(cur.series.Len() > 0 &&
			cur.series.Front().Value.(*TimeValues).time <= cur.series.Back().Value.(*TimeValues).time-int64(cur.intervalSec)) {
		Sub(cur.Statistics, cur.series.Front().Value.(*TimeValues).values)
		cur.series.Remove(cur.series.Front())
	}
}

type TimeSeriesWriter interface {
	InsertWithUserCfg(key string, time int64, values []float64, cfgIntervalSec int) bool /* is_success */
}

// // TODO depricated
// func (cur *TimeSeriesContainer) Insert(key string, time int64, values []float64) bool /* is_success */ {
// 	cur.mu.Lock()
// 	defer cur.mu.Unlock()
// 	val, ok := cur.seriesMap[key]
// 	if !ok {
// 		val = &SimpleTimeSeries{
// 			series:      list.New(),
// 			Statistics:  make([]AvgSigma, CapacityOfStaticsAvgSigma),
// 			cap:         cur.defaultCapOfSeries, /// TODO , assign from user's config
// 			intervalSec: cur.defaultCapOfSeries * MetricResolutionSeconds,
// 		}
// 		cur.seriesMap[key] = val
// 	}
// 	if time > val.max_time { // only insert point with larger timestamp
// 		val.append(time, values)
// 		return true
// 	} else {
// 		return false
// 	}
// }

type MetricsTopic int

const (
	MetricsTopicCpu     = MetricsTopic(0)
	MetricsTopicTaskCnt = MetricsTopic(1)
)

func (c *MetricsTopic) String() string {
	switch *c {
	case MetricsTopicCpu:
		return "cpu"
	case MetricsTopicTaskCnt:
		return "taskcnt"
	}
	return "others"
}

func (cur *TimeSeriesContainer) SeriesMap(metricsTopic MetricsTopic) map[string]*SimpleTimeSeries {
	if metricsTopic == MetricsTopicCpu {
		return cur.seriesMap
	} else if metricsTopic == MetricsTopicTaskCnt {
		return cur.taskCntSeriesMap
	} else {
		return nil
	}
}

func (cur *TimeSeriesContainer) CommonInsertWithUserCfg(key string, time int64, values []float64, cfgIntervalSec int, metricsTopic MetricsTopic) bool /* is_success */ {
	cur.mu.Lock()
	defer cur.mu.Unlock()
	seriesMap := cur.SeriesMap(metricsTopic)
	// if seriesMap == nil {
	// 	return false
	// }
	val, ok := seriesMap[key]

	if !ok {
		val = &SimpleTimeSeries{
			series:      list.New(),
			Statistics:  make([]AvgSigma, CapacityOfStaticsAvgSigma),
			cap:         computeSeriesCapBasedOnIntervalSec(cfgIntervalSec), /// TODO , assign from user's config
			intervalSec: cfgIntervalSec,
		}
		seriesMap[key] = val
	} else {
		if val.intervalSec != cfgIntervalSec {
			// reload cfgIntervalSec
			val.ReloadCfg(cfgIntervalSec)
		}
	}
	if time > val.max_time {
		val.append(time, values)
		return true
	} else {
		return false
	}
}

func (cur *TimeSeriesContainer) InsertTaskCntWithUserCfg(key string, time int64, values []float64, cfgIntervalSec int) bool /* is_success */ {

	return cur.CommonInsertWithUserCfg(key, time, values, cfgIntervalSec, MetricsTopicTaskCnt)
}

func (cur *TimeSeriesContainer) InsertWithUserCfg(key string, time int64, values []float64, cfgIntervalSec int) bool /* is_success */ {

	return cur.CommonInsertWithUserCfg(key, time, values, cfgIntervalSec, MetricsTopicCpu)
	// cur.mu.Lock()
	// defer cur.mu.Unlock()
	// val, ok := cur.seriesMap[key]
	// if !ok {
	// 	val = &SimpleTimeSeries{
	// 		series:      list.New(),
	// 		Statistics:  make([]AvgSigma, CapacityOfStaticsAvgSigma),
	// 		cap:         computeSeriesCapBasedOnIntervalSec(cfgIntervalSec), /// TODO , assign from user's config
	// 		intervalSec: cfgIntervalSec,
	// 	}
	// 	cur.seriesMap[key] = val
	// } else {
	// 	if val.intervalSec != cfgIntervalSec {
	// 		// reload cfgIntervalSec
	// 		val.ReloadCfg(cfgIntervalSec)
	// 	}
	// }
	// if time > val.max_time {
	// 	val.append(time, values)
	// 	return true
	// } else {
	// 	return false
	// }
}

type PromClient struct {
	cli api.Client
}

func NewPromClientDefault() (*PromClient, error) {
	client, err := api.NewClient(api.Config{
		Address: "http://as-prometheus.tiflash-autoscale.svc.cluster.local:16292",
	})
	if err != nil {
		Logger.Errorf("[error][PromClient] creating client: %v", err)
		return nil, err
	}
	return &PromClient{cli: client}, nil
}

func NewPromClient(addr string) (*PromClient, error) {
	client, err := api.NewClient(api.Config{
		Address: addr,
	})
	if err != nil {
		Logger.Errorf("[error][PromClient] creating client: %v", err)
		return nil, err
	}
	return &PromClient{cli: client}, nil
}

func promplay() {
	client, err := api.NewClient(api.Config{
		Address: "http://as-prometheus.tiflash-autoscale.svc.cluster.local:16292",
	})
	if err != nil {
		Logger.Errorf("Error creating client: %v", err)
		os.Exit(1)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, warnings, err := v1api.Query(ctx, "up", time.Now(), v1.WithTimeout(5*time.Second))
	if err != nil {
		Logger.Errorf("Error querying Prometheus: %v", err)
		os.Exit(1)
	}
	if len(warnings) > 0 {
		Logger.Infof("Warnings: %v", warnings)
	}
	Logger.Infof("Result:\n%v", result)
}

type TimeValPairVector struct {
	Vector []TimeValPair
}

// / TODO !!! we shoudn't direct use the result of "group by pod" since this pod may served many tenants in the past,
//
//	we can cut off the other tenants history in the series
func (c *PromClient) RangeQueryCpu(scaleInterval time.Duration, step time.Duration, tInfoProvider TenantInfoProvider, writer TimeSeriesWriter) (map[string]int, error) {
	// client, err := api.NewClient(api.Config{
	// 	Address: "http://localhost:16292",
	// })
	// if err != nil {
	// 	Logger.Infof("Error creating client: %v", err)
	// 	os.Exit(1)
	// }

	v1api := v1.NewAPI(c.cli)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now()
	r := v1.Range{
		Start: now.Add(-scaleInterval),
		End:   now,
		Step:  step,
	}
	// result, warnings, err := v1api.Query(ctx, "container_cpu_usage_seconds_total{job=\"kube_sd\", metrics_topic!=\"\", pod!=\"\"}[1m]", time.Now(), v1.WithTimeout(5*time.Second))
	result, warnings, err := v1api.QueryRange(ctx, "avg by(pod) (irate(container_cpu_usage_seconds_total{job=\"kube_sd\", pod=~\"readnode.+\"}[1m]))", r, v1.WithTimeout(5*time.Second))
	if err != nil {
		Logger.Errorf("Error querying Prometheus: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		Logger.Warnf("Warnings: %v", warnings)
	}
	Logger.Infof("[RangeQueryCpu]Result:\n%v", result)

	ret := make(map[string]int)
	matrix, ok := result.(model.Matrix)
	if ok {
		for _, sampleStream := range matrix {
			podName := sampleStream.Metric["pod"]
			// Logger.Infof("pod: %v", podName)
			// lenOfVals := len(sampleStream.Values)
			tenantName, sTimeOfAssign := tInfoProvider.GetTenantInfoOfPod(string(podName))
			if tenantName == "" {
				continue
			}
			scaleIntervalSec, hasErr := tInfoProvider.GetTenantScaleIntervalSec(tenantName)
			if hasErr {
				Logger.Errorf("[error][RangeQueryCpu]GetTenantScaleIntervalSec fail, there's no such tenant, tenant:%v", tenantName)
				continue
			}
			if tenantName == "" || sTimeOfAssign == 0 {
				if tenantName != "" && sTimeOfAssign == 0 {
					Logger.Errorf("[error][RangeQueryCpu]impossible branch, tenantName not empty and sTimeOfAssign is 0, tenant:%v", tenantName)
				}
				continue
			}
			cnt := 0
			for _, valCopy := range sampleStream.Values {
				curTime := valCopy.Timestamp.Unix()
				if curTime < sTimeOfAssign {
					continue
				}
				writer.InsertWithUserCfg(string(podName), curTime, []float64{
					float64(valCopy.Value),
					0.0, //TODO remove this dummy mem metric
				}, scaleIntervalSec)
				cnt++
			}
			ret[string(podName)] = cnt

		}
	} else {
		Logger.Errorf("[error][RangeQueryCpu]type cast fail when query cpu, real result:%v ", result)
	}
	return ret, nil
}

// query recent 1m , and get latest two cumulative value, and compute delta of them
func (c *PromClient) QueryCpu() (map[string]*TimeValPair, error) {
	// client, err := api.NewClient(api.Config{
	// 	Address: "http://localhost:16292",
	// })
	// if err != nil {
	// 	Logger.Infof("Error creating client: %v", err)
	// 	os.Exit(1)
	// }

	v1api := v1.NewAPI(c.cli)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// result, warnings, err := v1api.Query(ctx, "container_cpu_usage_seconds_total{job=\"kube_sd\", metrics_topic!=\"\", pod!=\"\"}[1m]", time.Now(), v1.WithTimeout(5*time.Second))
	result, warnings, err := v1api.Query(ctx, "container_cpu_usage_seconds_total{job=\"kube_sd\", pod=~\"readnode.+\"}[1m]", time.Now(), v1.WithTimeout(5*time.Second))
	if err != nil {
		Logger.Errorf("[error][PromClient] querying Prometheus error: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		Logger.Warnf("[warn][PromClient] Warnings: %v", warnings)
	}
	// Logger.Infof("[QueryCpu]Result: %v", result.String())
	// matrix := result.(model.Matrix)
	matrix, ok := result.(model.Matrix)
	ret := make(map[string]*TimeValPair)
	if ok {
		for _, sampleStream := range matrix {
			podName := sampleStream.Metric["pod"]
			// Logger.Infof("pod: %v", podName)
			lenOfVals := len(sampleStream.Values)
			if lenOfVals >= 2 {
				last := sampleStream.Values[lenOfVals-1]
				nextToLast := sampleStream.Values[lenOfVals-2]
				rate := float64(last.Value-nextToLast.Value) / float64(last.Timestamp.Unix()-nextToLast.Timestamp.Unix())
				v, ok := ret[string(podName)]

				if !ok || (ok && last.Timestamp.Unix() > v.time) { // there many be many series for a same podName, since label may be different
					ret[string(podName)] = &TimeValPair{
						time:  last.Timestamp.Unix(),
						value: rate,
					}
					Logger.Infof("[Prom]query cpu, key: %v time: %v val: %v", podName, last.Timestamp.Unix(), rate)
				}

			} else {
				Logger.Warnf("[warn][Prom]no enough points, pod:%v", podName)
			}
		}
	} else {
		Logger.Errorf("[error][Prom]type cast fail when query cpu, real result:%v ", result)
	}

	Logger.Infof("[Prom]query cpu, ret: %v, size:%v ", ret, len(ret))
	return ret, nil
}

func (c *PromClient) QueryComputeTask() (map[string]*TimeValPair, error) {

	v1api := v1.NewAPI(c.cli)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, warnings, err := v1api.Query(ctx, "sum by(pod) (sum_over_time(tiflash_coprocessor_handling_request_count{job=\"kube_sd_tiflash_proc\",metrics_topic=\"tiflash\", pod!=\"\"}[30s]))", time.Now(), v1.WithTimeout(5*time.Second))
	if err != nil {
		Logger.Errorf("[error][PromClient] querying Prometheus error: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		Logger.Warnf("[warn][PromClient] Warnings: %v", warnings)
	}
	// Logger.Infof("[QueryComputeTask]Result: %v", result.String())

	vector, ok := result.(model.Vector)
	ret := make(map[string]*TimeValPair)
	if ok {
		for _, sample := range vector {

			podName := sample.Metric["pod"]
			// Logger.Infof("pod: %v", podName)
			// lenOfVals := len(sampleStream.Values)

			ret[string(podName)] = &TimeValPair{
				time:  sample.Timestamp.Unix(),
				value: float64(sample.Value),
			}
			Logger.Infof("[Prom]query compute_task_cnt, key: %v time: %v val: %v", podName, sample.Timestamp.Unix(), float64(sample.Value))
		}
	} else {
		Logger.Errorf("[error][Prom]type cast fail when query compute_task_cnt, real result:%v ", result)
	}

	Logger.Infof("[Prom]query compute_task_cnt, ret: %v, size:%v ", ret, len(ret))
	return ret, nil
}

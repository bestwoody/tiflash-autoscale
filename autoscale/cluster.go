package autoscale

import (
	"context"
	"flag"
	"fmt"
	"math"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openkruise/kruise-api/apps/v1alpha1"
	kruiseclientset "github.com/openkruise/kruise-api/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	RunModeServeless = iota
	RunModeLocal
	RunModeDedicated
	RunModeCustom
	RunModeTest
)

const LabelNameForAlarm = "app.kubernetes.io/component"
const LabelValueForAlarm = "tiflash-autoscale"

const AnnotationKeyOfSupervisorRDVersionn = "tiflash.autoscale.rdversion"

func GetSupervisorDockerImager() string {
	if HardCodeSupervisorImage != "" {
		return HardCodeSupervisorImage
	} else {
		// if OptionRunMode == RunModeServeless {
		// 	return "bestwoody/supervisor:serverless.v1.0"
		// } else if OptionRunMode == RunModeLocal {
		return "bestwoody/supervisor:1"
		// } else { // TODO
		// 	return "bestwoody/supervisor:1"
		// }
	}
}

func GetBusyBoxDockerImager() string {
	// if HardCodeBusyBoxImage != "" {
	// 	return HardCodeBusyBoxImage
	// } else {
	// if OptionRunMode == RunModeServeless {
	// 	return "bestwoody/supervisor:serverless.v1.0"
	// } else if OptionRunMode == RunModeLocal {
	return "gcr.io/pingcap-public/dbaas/busybox:1.31.1"
	// } else { // TODO
	// 	return "bestwoody/supervisor:1"
	// }
	// }
}

const AutoScaleNamespace = "tiflash-autoscale"
const ReadNodeCloneSetName = "readnode"

var OptionRunMode = RunModeServeless
var EnvRegion string

func RunModeEnvString4Supervisor() string {
	if OptionRunMode == RunModeDedicated {
		return "dedicated"
	} else if OptionRunMode == RunModeServeless {
		return "serverless"
	} else {
		return "local"
	}
}

func outsideConfig() (*restclient.Config, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	return clientcmd.BuildConfigFromFlags("", *kubeconfig)
}

func getK8sConfig() (*restclient.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return outsideConfig()
	} else {
		return config, err
	}
}

// TODO mutex protection
type ClusterManager struct {
	Namespace     string
	CloneSetName  string
	SnsManager    *AwsSnsManager
	PromClient    *PromClient
	AutoScaleMeta *AutoScaleMeta
	ConfigManager *ConfigManager
	K8sCli        kubernetes.Interface
	MetricsCli    metricsv.Interface
	Cli           kruiseclientset.Interface
	CloneSet      *v1alpha1.CloneSet
	wg            sync.WaitGroup
	shutdown      int32 // atomic
	watchMu       sync.Mutex
	watcher       watch.Interface
	muOfCloneSet  sync.Mutex

	ExternalFixPoolReplica atomic.Int32 //info about fixpool used in dev tier,
	tsContainer            *TimeSeriesContainer
	lstTsMap               map[string]int64 // TODO remove it
	analyzeTaskMap         sync.Map         //map[string]*AnalyzeTask
}

// cnt: want, create, get

// TODO expire of removed Pod in tsContainer,lstTsMap

func (c *ClusterManager) collectMetricsFromMetricServerLoop() {
	c.collectMetricsLoop(MetricsTopicCpu, true)
}

// checked
func (c *ClusterManager) initRangeMetricsFromPromethues(intervalSec int) error {
	as_meta := c.AutoScaleMeta
	tsContainer := c.tsContainer

	Logger.Infof("[initRangeMetricsFromPromethues] range query cpu")
	_, err := c.PromClient.RangeQueryCpu(time.Duration(intervalSec)*time.Second, 15*time.Second, c.AutoScaleMeta, c.tsContainer)
	if err != nil {
		Logger.Errorf("[error][initRangeMetricsFromPromethues]QueryCpu fail, err:%v", err.Error())
		return err
	}

	//print debug info
	tsContainer.DumpAll(MetricsTopicCpu)
	tArr := c.AutoScaleMeta.GetTenantNames()
	for _, tName := range tArr {
		stats, podCpuMap, podPointCntMap, _, _ := as_meta.ComputeStatisticsOfTenant(tName, tsContainer, "collectMetrics", MetricsTopicCpu)
		Logger.Infof("[initRangeMetricsFromPromethues]Tenant %v statistics: cpu: %v %v mem: %v %v, cpuMap:%+v valPointMap:%+v ", tName,
			stats[0].Avg(),
			stats[0].Cnt(),
			stats[1].Avg(),
			stats[1].Cnt(),
			podCpuMap,
			podPointCntMap,
		)
	}

	return nil

}

func (c *ClusterManager) collectTaskCntMetricsFromPromethuesLoop() {
	c.collectMetricsLoop(MetricsTopicTaskCnt, false)
}

// checked
func (c *ClusterManager) collectMetricsLoop(metricsTopic MetricsTopic, fromMetricServer bool) {
	c.wg.Add(1)
	defer c.wg.Done()
	as_meta := c.AutoScaleMeta
	tsContainer := c.tsContainer
	lastQueryTs := int64(0)
	collectIntervalSec := int64(15)
	for {
		if time.Now().Unix() < lastQueryTs+collectIntervalSec {
			time.Sleep(time.Second)
			continue
		}

		if atomic.LoadInt32(&c.shutdown) != 0 {
			return
		}
		Logger.Infof("[collectMetrics] query %v, fromMetricServer: %v", metricsTopic.String(), fromMetricServer)
		lastQueryTs = time.Now().Unix()
		var metricOfPods map[string]*TimeValPair
		var err error

		if metricsTopic == MetricsTopicCpu {
			if fromMetricServer {
				labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": c.CloneSetName}}
				// st := time.Now().UnixNano()
				podMetricsList, err := c.MetricsCli.MetricsV1beta1().PodMetricses(c.Namespace).List(
					context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
				if err == nil {
					metricOfPods = make(map[string]*TimeValPair)
					for _, pod := range podMetricsList.Items {
						metricOfPods[pod.Name] = &TimeValPair{
							time:  pod.Timestamp.Unix(),
							value: pod.Containers[0].Usage.Cpu().AsApproximateFloat64(),
						}
					}
				}
			} else {
				metricOfPods, err = c.PromClient.QueryCpu()
			}
		} else if metricsTopic == MetricsTopicTaskCnt {
			metricOfPods, err = c.PromClient.QueryComputeTask()
		} else {
			panic(fmt.Errorf("unknown MetricsTopic:%v", metricsTopic))
		}

		if err != nil {
			Logger.Errorf("[error][collectMetrics]fail to query metric:%v fromMetricServer:%v", metricsTopic.String(), fromMetricServer)
			continue
		}

		mint := int64(math.MaxInt64)
		maxt := int64(0)

		for podName, metric := range metricOfPods {
			tenantName, _ := as_meta.GetTenantInfoOfPod(podName)
			if tenantName == "" { //prewarm pod or dead pod
				continue
			}
			tenantDesc := as_meta.GetTenantDesc(tenantName)
			if tenantDesc == nil {
				Logger.Errorf("[error][collectMetrics]tenantdesc is nil, tenant:%v", tenantName)
				continue
			}
			if metricsTopic == MetricsTopicCpu {
				tsContainer.InsertWithUserCfg(podName, metric.time,
					[]float64{
						metric.value,
						0.0, //TODO remove this dummy mem metric
					}, tenantDesc.GetScaleIntervalSec(), MetricsTopicCpu)
			} else if metricsTopic == MetricsTopicTaskCnt {
				autoPauseIntervalSeconds := tenantDesc.GetAutoPauseIntervalSec()
				if autoPauseIntervalSeconds == 0 {
					Logger.Infof("[collectMetrics]tenant %v 's auto-pause is disabled", tenantName)
				} else {
					tsContainer.InsertWithUserCfg(podName, metric.time,
						[]float64{
							metric.value,
							0.0, //TODO remove this dummy mem metric
						}, autoPauseIntervalSeconds, MetricsTopicTaskCnt)
				}
			} else {
				panic(fmt.Errorf("unknown MetricsTopic#2:%v", metricsTopic))
			}

			snapshot := tsContainer.GetSnapshotOfTimeSeries(podName, metricsTopic)
			if snapshot != nil {
				mint = Min(snapshot.MinTime, mint)
				maxt = Max(snapshot.MaxTime, maxt)
			} else {
				Logger.Infof("[collectMetrics]GetSnapshotOfTimeSeries: snapshot is nil! tenant:%v pod:%v ", tenantDesc.Name, podName)
			}

		}

		// just print tenant's avg metrics
		tArr := c.AutoScaleMeta.GetTenantNames()
		for _, tName := range tArr {
			stats, _, _, _, _ := as_meta.ComputeStatisticsOfTenant(tName, tsContainer, "collectTaskCntMetricsFromPromethues", metricsTopic)
			if stats != nil {
				var statVal float64
				if metricsTopic == MetricsTopicCpu {
					statVal = stats[0].Avg()
				} else if metricsTopic == MetricsTopicTaskCnt {
					statVal = stats[0].Sum()
				} else {
					panic(fmt.Errorf("unknown MetricsTopic#3:%v", metricsTopic))
				}
				Logger.Infof("[collectMetrics]metricsTopic:%v Tenant %v statistics: val, cnt: %v %v time_range:%v~%v",
					metricsTopic.String(), tName,
					statVal,
					stats[0].Cnt(),
					mint, maxt,
				)
			} else {
				Logger.Infof("[collectMetrics]ComputeStatisticsOfTenant: stats is nil! metricsTopic:%v tenant:%v ",
					metricsTopic.String(), tName)
			}
		}
		tsContainer.DumpAll(metricsTopic)
	}
}

func (c *ClusterManager) collectMetricsFromPromethuesLoop() {
	c.collectMetricsLoop(MetricsTopicCpu, false)
}

type AnalyzeTask struct {
	tenant              *TenantDesc
	endSyn              atomic.Bool
	endFin              atomic.Bool
	refOfAnalyzeTaskMap *sync.Map //map[string]*AnalyzeTask
}

func NewAnalyzeTask(tenant *TenantDesc, refOfAnalyzeTaskMap *sync.Map, c *ClusterManager) *AnalyzeTask {
	ret := &AnalyzeTask{tenant: tenant, refOfAnalyzeTaskMap: refOfAnalyzeTaskMap}
	go ret.analyzeTaskLoop(c)
	return ret
}

func (c *AnalyzeTask) Shutdown() bool {
	if !c.endSyn.Load() {
		c.endSyn.Store(true)
		return true
	}
	return false
}

// checked
func (task *AnalyzeTask) analyzeTaskLoop(c *ClusterManager) {
	lastTs := int64(0)
	loopIntervalSec := int64(10)
	for !task.endSyn.Load() {
		roundBeginTime := time.Now()
		if roundBeginTime.Unix() < lastTs+loopIntervalSec {
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		lastTs = roundBeginTime.Unix()
		tenant := task.tenant

		Logger.Debugf("[analyzeTaskLoop][%v]round begin. tenant: %v", tenant.Name, tenant.Name)
		tenant.TryToReloadConf(false) /// TODO implement

		cntOfPods := tenant.GetCntOfPods()
		if tenant.IsDisabled() {
			ok := c.AsyncPause(tenant.Name)
			Logger.Infof("[analyzeTaskLoop][%v]tenant disabled, try to pause return %v, cntOfPods:%v ", tenant.Name, ok, cntOfPods)
			continue
		}

		isStateCorrect, expectedStateIfIncorrect := tenant.IsStateCorrect()
		if !isStateCorrect {
			Logger.Errorf("[analyzeTaskLoop][%v] incorrect state:%v expect_state:%v", tenant.Name, TenantState2String(tenant.GetState()), TenantState2String(expectedStateIfIncorrect))
			if expectedStateIfIncorrect == TenantStateResumed {
				c.Resume(tenant.Name)
			} else if expectedStateIfIncorrect == TenantStatePaused {
				c.AsyncPause(tenant.Name)
			}
			continue
		}

		//anto scale/pause analyze of tenant
		if tenant.GetState() != TenantStateResumed { // tenant not available
			Logger.Infof("[analyzeTaskLoop][%v]tenant's state is not resumed! current:%v", tenant.Name, tenant.GetState())
			continue
		}

		// PreCondition of Analytics:
		//    1. now - max(startTimeOfAssignOfPod) >= AnalyzeInterval(scale/autopuase)
		//    2. at least 2 metric points for each pod (since scale in minute resolution, and collect metrics in about 15s resolution)
		//    3. minTime of metric points of each pod should meet condition:  now - AnalyzeInterval < minTime < now - AnalyzeInterval + 30s

		// Auto Pause
		autoPauseIntervalSec := tenant.GetAutoPauseIntervalSec()
		if autoPauseIntervalSec != 0 { // if auto-pause is on
			taskCntStats, _, _, _, tenantMetricDesc := c.AutoScaleMeta.ComputeStatisticsOfTenant(tenant.Name, c.tsContainer, "AutoPauseAnalytics", MetricsTopicTaskCnt)
			if taskCntStats != nil {
				// autoPauseIntervalSec := tenant.GetAutoPauseIntervalSec()
				now := time.Now().Unix()
				if tenantMetricDesc.MinOfPodTimeseriesSize >= 2 && tenantMetricDesc.MaxOfPodMinTime < now-int64(autoPauseIntervalSec)+30 {
					totalTaskCnt := taskCntStats[0].Sum()
					if totalTaskCnt < 1 { //test is zero, since it's a float, "< 1" may be better
						Logger.Infof("[analyzeTaskLoop][%v]auto pause, tenant: %v MinOfPodTimeseriesSize:%v MinOfMetricInterval:%v AutoPauseIntervalSec:%v   ", tenant.Name, tenant.Name, tenantMetricDesc.MinOfPodTimeseriesSize, now-tenantMetricDesc.MaxOfPodMinTime, autoPauseIntervalSec)
						c.AsyncPause(tenant.Name)
						continue //skip auto scale
					}
				} else {
					Logger.Warnf("[analyzeTaskLoop][%v]condition of auto pause haven't not met, tenant: %v MinOfPodTimeseriesSize:%v MinOfMetricInterval:%v AutoPauseIntervalSec:%v   ", tenant.Name,
						tenant.Name, tenantMetricDesc.MinOfPodTimeseriesSize, now-tenantMetricDesc.MaxOfPodMinTime, autoPauseIntervalSec)
				}

			} else {
				Logger.Errorf("[error][analyzeTaskLoop][%v]empty metric: TaskCnt , tenant: %v", tenant.Name, tenant.Name)
			}
		}

		// Auto Scale
		cntOfPods = tenant.GetCntOfPods()
		if cntOfPods < tenant.GetMinCntOfPod() {
			Logger.Infof("[analyzeTaskLoop][%v] StateResume and cntOfPods < tenant.MinCntOfPod, add more pods if curCntofPods != 0, curCntofPods:%v minCntOfPods:%v tenant: %v", tenant.Name, cntOfPods, tenant.GetMinCntOfPod(), tenant.Name)
			c.AutoScaleMeta.ResizePodsOfTenant(cntOfPods, tenant.GetInitCntOfPod(), tenant.Name, c.tsContainer)
			if c.SnsManager != nil {
				c.SnsManager.TryToPublishTopology(tenant.Name, time.Now().UnixNano(), tenant.GetPodNames()) // public latest topology into SNS
			}
		} else {
			stats, podCpuMap, _, _, tenantMetricDesc := c.AutoScaleMeta.ComputeStatisticsOfTenant(tenant.Name, c.tsContainer, "analyzeMetrics", MetricsTopicCpu)
			/// TODO use tenantMetricDesc to check preCondition of auto scale of this tenant
			autoScaleIntervalSec := tenant.GetScaleIntervalSec()
			now := time.Now().Unix()
			podCnt := tenant.GetCntOfPods()
			Logger.Debugf("[analyzeTaskLoop][%v]condition of auto scale, metricPodCnt:%v podCnt:%v MinOfPodTimeseriesSize:%v MinOfMetricInterval:%v AutoScaleIntervalSec:%v   ", tenant.Name,
				tenantMetricDesc.PodCnt, podCnt,
				tenantMetricDesc.MinOfPodTimeseriesSize, now-tenantMetricDesc.MaxOfPodMinTime, autoScaleIntervalSec)
			if stats != nil && tenantMetricDesc.MinOfPodTimeseriesSize >= 2 && tenantMetricDesc.MaxOfPodMinTime < now-int64(autoScaleIntervalSec)+30 {
				cpuusage := stats[0].Avg()

				minCpuUsageThreshold, maxCpuUsageThreshold := tenant.GetLowerAndUpperCpuScaleThreshold()
				bestPods, _ := ComputeBestPodsInRuleOfCompute(tenant, cpuusage, minCpuUsageThreshold, maxCpuUsageThreshold)

				Logger.Infof("[analyzeTaskLoop][%v]ComputeStatisticsOfTenant, Tenant %v , cpu usage: %v %v , PodsCpuMap: %+v bestPods: %v min_max_cpu_threshold: %v~%v ", tenant.Name, tenant.Name,
					stats[0].Avg(), stats[0].Cnt(), podCpuMap, bestPods, minCpuUsageThreshold, maxCpuUsageThreshold)
				if bestPods != -1 && cntOfPods != bestPods {
					Logger.Infof("[analyzeTaskLoop][%v] resize pods, from %v to  %v , tenant: %v", tenant.Name, tenant.GetCntOfPods(), bestPods, tenant.Name)
					c.AutoScaleMeta.ResizePodsOfTenant(cntOfPods, bestPods, tenant.Name, c.tsContainer)
					if c.SnsManager != nil {
						c.SnsManager.TryToPublishTopology(tenant.Name, time.Now().UnixNano(), tenant.GetPodNames()) // public latest topology into SNS
					}
				} else {
					// unchanged

					// Logger.Infof("[analyzeMetrics] pods unchanged cnt:%v, bestCnt:%v, tenant:%v ", tenant.GetCntOfPods(), bestPods, tenant.Name)
				}
			} else {
				if stats == nil {
					Logger.Errorf("[error][analyzeTaskLoop][%v]empty metric: CPU , tenant: %v", tenant.Name, tenant.Name)
				} else {
					Logger.Warnf("[analyzeTaskLoop][%v]condition of auto scale haven't not met, metricPodCnt:%v podCnt:%v MinOfPodTimeseriesSize:%v MinOfMetricInterval:%v AutoScaleIntervalSec:%v   ", tenant.Name,
						tenantMetricDesc.PodCnt, podCnt, tenantMetricDesc.MinOfPodTimeseriesSize, now-tenantMetricDesc.MaxOfPodMinTime, autoScaleIntervalSec)
				}
			}
		}

		Logger.Debugf("[analyzeTaskLoop][%v]round end. tenant: %v , cost %vms", tenant.Name, tenant.Name, time.Now().UnixMilli()-roundBeginTime.UnixMilli())
	}
	task.endFin.Store(true)
	task.refOfAnalyzeTaskMap.Delete(task.tenant.Name)
}

// checked
func (c *ClusterManager) manageAnalyzeTasks() {
	c.wg.Add(1)
	// c.tsContainer.GetSnapshotOfTimeSeries()
	defer c.wg.Done()
	lastTs := int64(0)
	loopIntervalSec := int64(10)
	for {
		roundBeginTime := time.Now()
		if roundBeginTime.Unix() < lastTs+loopIntervalSec {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if atomic.LoadInt32(&c.shutdown) != 0 {
			// shut down all analyze-tasks
			c.analyzeTaskMap.Range(func(k, v interface{}) bool {
				v.(*AnalyzeTask).Shutdown()
				return true
			})
			return
		}

		lastTs = roundBeginTime.Unix()
		tenants := c.AutoScaleMeta.GetTenants()
		Logger.Infof("[manageAnalyzeTasks]round begin. tenants cnt: %v", len(tenants))
		tenantSet := make(map[string]bool) // for print
		// create tasks for new tenants
		crtCnt := 0
		delCnt := 0
		for _, tenant := range tenants {
			tenantSet[tenant.Name] = true
			_, ok := c.analyzeTaskMap.Load(tenant.Name)
			if !ok {
				// new analyze task
				crtCnt++
				c.analyzeTaskMap.Store(tenant.Name, NewAnalyzeTask(tenant, &c.analyzeTaskMap, c))
			}
		}
		// shutdown tasks of removed tenants
		c.analyzeTaskMap.Range(func(k, v interface{}) bool {
			_, ok := tenantSet[k.(string)]
			if !ok {
				delCnt++
				v.(*AnalyzeTask).Shutdown()
			}
			return true
		})

		Logger.Infof("[manageAnalyzeTasks]round end. tenants cnt: %v , cnt_of_new_tenants: %v cnt_of_del_tenents:%v, cost %vms", len(tenants), crtCnt, delCnt, time.Now().UnixMilli()-roundBeginTime.UnixMilli())
	}
}

func Int32Ptr(val int32) *int32 {
	ret := new(int32)
	*ret = int32(val)
	return &val
}

func (c *ClusterManager) Shutdown() {
	Logger.Infof("[ClusterManager]Shutdown")
	atomic.StoreInt32(&c.shutdown, 1)
	c.watchMu.Lock()
	c.watcher.Stop()
	c.watchMu.Unlock()
	c.wg.Wait()
}

func (c *ClusterManager) AsyncPause(tenant string) bool {
	return c.AutoScaleMeta.AsyncPause(tenant, c.tsContainer)
}

func (c *ClusterManager) Resume(tenant string) bool {
	// resultChan := make(chan int)
	resultChan, ret := c.AutoScaleMeta.AsyncResume(tenant, c.tsContainer)
	addPodsResult := int(-1)
	if resultChan != nil {
		addPodsResult = <-resultChan
	}
	return ret && addPodsResult != -1
}

// checked
func (c *ClusterManager) watchPodsLoop(resourceVersion string) {
	defer c.wg.Done()
	msgid := 0
	for {
		if atomic.LoadInt32(&c.shutdown) != 0 {
			return
		}
		labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": c.CloneSetName}}
		watcher, err := c.K8sCli.CoreV1().Pods(c.Namespace).Watch(context.TODO(),
			metav1.ListOptions{
				LabelSelector:   labels.Set(labelSelector.MatchLabels).String(),
				ResourceVersion: resourceVersion,
			})

		if err != nil {
			Logger.Errorf("[watchPodsLoop]watch failed! err:%v", err.Error())
			time.Sleep(1 * time.Second)
			continue
			// panic(err.Error())
		}

		c.watchMu.Lock()
		c.watcher = watcher
		c.watchMu.Unlock()

		ch := watcher.ResultChan()

		// LISTEN TO CHANNEL
		for {
			e, more := <-ch
			if !more {
				Logger.Infof("[watchPodsLoop]watchPods channel closed")
				time.Sleep(1 * time.Second)
				resourceVersion = c.loadPods()
				break
			}
			pod, ok := e.Object.(*v1.Pod)
			if !ok {
				continue
			}
			msgid += 1
			Logger.Infof("[watchPodsLoop] receive new pod changes, pod:%v type:%v msgid:%v", pod.Name, e.Type, msgid)

			resourceVersion = pod.ResourceVersion
			switch e.Type {
			case watch.Added:
				c.AutoScaleMeta.UpdatePod(pod)
				MetricOfWatchPodsLoopEventAddedCnt.Inc()
			case watch.Modified:
				c.AutoScaleMeta.UpdatePod(pod)
				MetricOfWatchPodsLoopEventModifiedCnt.Inc()
			case watch.Deleted:
				c.AutoScaleMeta.HandleK8sDelPodEvent(pod.Name)
				MetricOfWatchPodsLoopEventDeletedCnt.Inc()
			case watch.Error:
				Logger.Error("[watchPodsLoop]watch.Error:%v", pod)
				MetricOfWatchPodsLoopEventErrorCnt.Inc()
			default:
				fallthrough
			case watch.Bookmark: //TODO handle it
				MetricOfWatchPodsLoopEventBookmarkCnt.Inc()
				continue
			}
			Logger.Infof("[watchPodsLoop] finish handle of new pod changes, pod:%v type:%v msgid:%v", pod.Name, e.Type, msgid)
		}
	}

}

// checked
func (c *ClusterManager) loadPods() string {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": c.CloneSetName}}
	pods, err := c.K8sCli.CoreV1().Pods(c.Namespace).List(context.TODO(),
		metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		Logger.Errorf("[loadPodsAtStartup] error, %v", err.Error())
		panic(err)
		// return ""
	}
	resVer := pods.ListMeta.ResourceVersion
	podSet := make(map[string]bool)
	for _, pod := range pods.Items {
		podSet[pod.Name] = true
		c.AutoScaleMeta.UpdatePod(&pod)
	}
	c.AutoScaleMeta.TryToRemoveExpriedPod(podSet)
	return resVer
}

// checked
func (c *ClusterManager) getComputePodAntiAffinity() *v1.PodAntiAffinity {
	if OptionRunMode == RunModeLocal || OptionRunMode == RunModeTest {
		return nil
	} else {
		return &v1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					TopologyKey: "kubernetes.io/hostname",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: "In",
								Values:   []string{c.CloneSetName, "autoscale"},
							},
						},
					},
				},
			},
		}
	}
}

func (c *ClusterManager) getComputePodToleration() []v1.Toleration {
	if OptionRunMode == RunModeServeless {
		return []v1.Toleration{
			{
				Key:      "tiflash.used-for-compute",
				Value:    "true",
				Operator: v1.TolerationOpEqual,
				Effect:   v1.TaintEffectNoSchedule,
			},
		}
	} else if OptionRunMode == RunModeDedicated {
		return []v1.Toleration{
			{
				Key:      "dedicated",
				Value:    "elastic-tiflash",
				Operator: v1.TolerationOpEqual,
				Effect:   v1.TaintEffectNoSchedule,
			},
		}
	} else {
		return nil
	}
}

func (c *ClusterManager) createCloneSet(cloneSet v1alpha1.CloneSet) (*v1alpha1.CloneSet, error) {
	Logger.Infof("[initK8sComponents]create clonneSet")
	c.AutoScaleMeta.PrewarmPool.cntOfPending.Add(*cloneSet.Spec.Replicas)
	return c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Create(context.TODO(), &cloneSet, metav1.CreateOptions{})
}

func (c *ClusterManager) getSupervisorRdVersion() string {
	if OptionRunMode == RunModeServeless {
		return "4"
	} else if OptionRunMode == RunModeDedicated { //dedicated tier
		return "2"
	} else {
		return "2"
	}
}

func (c *ClusterManager) getTiflashCachePath() string {
	if OptionRunMode == RunModeDedicated { //dedicated tier
		return "/data/cache"
	} else if OptionRunMode == RunModeServeless {
		return "/data/cache"
	} else {
		return ""
	}

}

func (c *ClusterManager) getTiflashCacheCap() string {
	if OptionRunMode == RunModeDedicated { //dedicated tier
		return "107374182400" // 100G
	} else if OptionRunMode == RunModeServeless {
		return "80530636800" // 75G
	} else {
		return "10737418240" //10G
	}
}

func (c *ClusterManager) getVolumesMount() []v1.VolumeMount {
	if OptionRunMode == RunModeDedicated { //dedicated tier
		return []v1.VolumeMount{
			{
				Name:      "sharedtmpdisk",
				MountPath: "/tiflash/log/",
			},

			{
				Name:      "cachedisk",
				MountPath: "/data",
			},
		}
	} else if OptionRunMode == RunModeServeless {
		return []v1.VolumeMount{
			{
				Name:      "sharedtmpdisk",
				MountPath: "/tiflash/log/",
			},

			{
				Name:      "cachedisk",
				MountPath: "/data",
			},
		}
	} else {
		return []v1.VolumeMount{
			{
				Name:      "sharedtmpdisk",
				MountPath: "/tiflash/log/",
			},
		}
	}
}

func (c *ClusterManager) getVolumeClaimTemplates() []v1.PersistentVolumeClaim {
	if OptionRunMode == RunModeServeless {
		scn := "ebs-sc"
		vm := v1.PersistentVolumeFilesystem
		return []v1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cachedisk",
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						"ReadWriteOnce",
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"storage": resource.MustParse("105Gi"),
						},
					},
					StorageClassName: &scn,
					VolumeMode:       &vm,
				},
			},
		}
	} else if OptionRunMode == RunModeDedicated { //dedicated tier
		scn := "local-storage"
		vm := v1.PersistentVolumeFilesystem
		return []v1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cachedisk",
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						"ReadWriteOnce",
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"storage": resource.MustParse("110Gi"),
						},
					},
					StorageClassName: &scn,
					VolumeMode:       &vm,
				},
			},
		}
	} else {
		return nil
	}
}

// checked
// TODO pod storage volume
func (c *ClusterManager) initK8sComponents() {
	// create cloneset if not exist
	cloneSetList, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	Logger.Infof("list clonneSet: %+v ", len(cloneSetList.Items))
	found := false
	for _, cloneSet := range cloneSetList.Items {
		if cloneSet.Name == c.CloneSetName {
			found = true
			break
		}
	}
	var retCloneset *v1alpha1.CloneSet
	desiredCloneSet := v1alpha1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.CloneSetName,
			Labels: map[string]string{
				"app":             c.CloneSetName,
				LabelNameForAlarm: LabelValueForAlarm,
			},
			Annotations: map[string]string{
				AnnotationKeyOfSupervisorRDVersionn: c.getSupervisorRdVersion(),
			},
		},
		Spec: v1alpha1.CloneSetSpec{
			Replicas: Int32Ptr(int32(c.AutoScaleMeta.SoftLimit)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": c.CloneSetName,
				}},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":             c.CloneSetName,
						LabelNameForAlarm: LabelValueForAlarm,
					},
					Annotations: map[string]string{
						"prometheus.io/path":   "/metrics",
						"prometheus.io/port":   "8234",
						"prometheus.io/scrape": "true",
					},
				},
				// pod anti affinity
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"tiflash.used-for-compute": "true",
						// "node.kubernetes.io/instance-type": "m6a.2xlarge", // TODO use a non-hack way to bind readnode pod to specific nodes
					},
					Tolerations: c.getComputePodToleration(),
					Affinity: &v1.Affinity{
						PodAntiAffinity: c.getComputePodAntiAffinity(),
					},
					ServiceAccountName: "default",
					// container
					Containers: []v1.Container{
						{
							// ENV
							Env: []v1.EnvVar{
								{
									Name: "POD_IP",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "S3_FOR_TIFLASH_LOG",
									Value: ReadNodeLogUploadS3Bucket,
								},
								{
									Name:  "TIFLASH_CACHE_PATH",
									Value: c.getTiflashCachePath(),
								},
								{
									Name:  "TIFLASH_CACHE_CAP",
									Value: c.getTiflashCacheCap(),
								},
								{
									Name:  "AS_RUN_MODE_ENV",
									Value: RunModeEnvString4Supervisor(),
								},
							},
							Name: "supervisor",
							// docker image
							Image:           GetSupervisorDockerImager(),
							ImagePullPolicy: "IfNotPresent",
							VolumeMounts:    c.getVolumesMount(),
						},
						/*
							- name: count-log-1
								image: busybox:1.28
								args: [/bin/sh, -c, 'tail -n+1 -F /var/log/1.log']
								volumeMounts:
								- name: varlog
								  mountPath: /var/log */
						{
							Name:            "tiflash-log",
							Image:           GetBusyBoxDockerImager(),
							ImagePullPolicy: "IfNotPresent",
							Args: []string{
								"/bin/sh",
								"-c",
								"touch /tiflash/log/tiflash.log; tail -n0 -F /tiflash/log/tiflash.log;",
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "sharedtmpdisk",
									MountPath: "/tiflash/log/",
								},
							},
						},
						{
							Name:            "tiflash-err-log",
							Image:           GetBusyBoxDockerImager(),
							ImagePullPolicy: "IfNotPresent",
							Args: []string{
								"/bin/sh",
								"-c",
								"touch /tiflash/log/tiflash_error.log; tail -n0 -F /tiflash/log/tiflash_error.log;",
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "sharedtmpdisk",
									MountPath: "/tiflash/log/",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "sharedtmpdisk",
							// EmptyDir: nil,
						},
					},
				},
			},
			VolumeClaimTemplates: c.getVolumeClaimTemplates(),
		},
	}
	if !found {
		//create cloneSet since there is no desired cloneSet
		retCloneset, err = c.createCloneSet(desiredCloneSet)
	} else {
		isCloneSetNeedRecreate := false
		Logger.Infof("get clonneSet")
		retCloneset, err = c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Get(context.TODO(), c.CloneSetName, metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}
		if retCloneset.ObjectMeta.Annotations != nil {
			v, ok := retCloneset.ObjectMeta.Annotations[AnnotationKeyOfSupervisorRDVersionn]
			if ok { // old annotation key exist
				if desiredCloneSet.ObjectMeta.Annotations != nil {
					v2, ok2 := desiredCloneSet.ObjectMeta.Annotations[AnnotationKeyOfSupervisorRDVersionn]
					if ok2 {
						if v2 != v {
							isCloneSetNeedRecreate = true
						}
					}
				}
			} else { // old annotation key not exist
				if desiredCloneSet.ObjectMeta.Annotations != nil {
					_, ok2 := desiredCloneSet.ObjectMeta.Annotations[AnnotationKeyOfSupervisorRDVersionn]
					if ok2 {
						isCloneSetNeedRecreate = true
					}
				}
			}
		} else { // old annotations not exist
			if desiredCloneSet.ObjectMeta.Annotations != nil {
				_, ok2 := desiredCloneSet.ObjectMeta.Annotations[AnnotationKeyOfSupervisorRDVersionn]
				if ok2 {
					isCloneSetNeedRecreate = true
				}
			}
		}
		Logger.Infof("[initK8sComponents]old annotations:%+v, new annotations:%+v", retCloneset.ObjectMeta.Annotations, desiredCloneSet.ObjectMeta.Annotations)
		if isCloneSetNeedRecreate {
			MaxRetryTimes := 100
			RetryIntervalSec := 1
			retryCnt := 0
			Logger.Infof("[initK8sComponents]recreate clonneSet")
			Logger.Infof("[initK8sComponents]delete clonneSet")
			err = c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Delete(context.TODO(), c.CloneSetName, metav1.DeleteOptions{})
			for err != nil && retryCnt < MaxRetryTimes {
				retryCnt++
				time.Sleep(time.Duration(RetryIntervalSec) * time.Second)
				Logger.Infof("[initK8sComponents][retry]delete clonneSet")
				err = c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Delete(context.TODO(), c.CloneSetName, metav1.DeleteOptions{})
			}
			if err == nil {
				retCloneset, err = c.createCloneSet(desiredCloneSet)
			}
		} else {
			expectedImage := GetSupervisorDockerImager()
			if expectedImage != retCloneset.Spec.Template.Spec.Containers[0].Image {
				Logger.Warnf("[initK8sComponents]image has changed! from: %v to %v", retCloneset.Spec.Template.Spec.Containers[0].Image, expectedImage)
				err = CloneSetPatchImage(c.Cli, c.Namespace, c.CloneSetName, expectedImage)
				if err != nil {
					panic(err.Error())
				}
				Logger.Infof("[initK8sComponents]image update done. from: %v to %v", retCloneset.Spec.Template.Spec.Containers[0].Image, expectedImage)
				retCloneset, err = c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Get(context.TODO(), c.CloneSetName, metav1.GetOptions{})
			}
		}
	}
	if err != nil {
		panic(err.Error())
	} else {
		c.CloneSet = retCloneset.DeepCopy()
	}

	// load k8s pods of cloneset
	resVer := c.loadPods()

	c.AutoScaleMeta.ScanStateOfPods(true)
	c.AutoScaleMeta.IsRuntimeReady.Store(true)

	// watch changes of pods
	c.wg.Add(2)

	go c.watchPodsLoop(resVer)

	// pod prepare & GC
	go c.podPrepareLoop()
}

func (c *ClusterManager) scanPodsStatesLoop() {
	c.wg.Add(1)
	defer c.wg.Done()
	periodSec := 60
	for {
		time.Sleep(time.Duration(periodSec) * time.Second)
		if atomic.LoadInt32(&c.shutdown) != 0 {
			return
		}
		c.AutoScaleMeta.ScanStateOfPods(false)
	}
}

func (c *ClusterManager) checkFixPoolReplicaLoop() {
	if OptionRunMode == RunModeServeless {
		Logger.Infof("[loop]checkFixPoolReplicaLoop begin")
		// defer Logger.Infof("[loop]checkFixPoolReplicaLoop end")
		fst := true
		c.wg.Add(1)
		defer c.wg.Done()
		periodSec := 60
		for {
			time.Sleep(time.Duration(periodSec) * time.Second)
			if atomic.LoadInt32(&c.shutdown) != 0 {
				return
			}
			specReplica, statusReplica, err := GetReplicaOfStatefulSet(c.K8sCli, FixPoolNameSpace, FixPoolRdName)

			if err != nil {
				Logger.Errorf("[checkFixPoolReplicaLoop]GetReplicaOfStatefulSet failed, err: %v", err.Error())
			} else {
				newReplica := specReplica
				if statusReplica != 0 {
					newReplica = MinInt(statusReplica, newReplica)
				}
				if fst {
					Logger.Infof("[checkFixPoolReplicaLoop]get replica at first time, replica: %v, spec: %v status: %v", newReplica, specReplica, statusReplica)
					fst = false
				}
				if newReplica >= 0 {
					c.ExternalFixPoolReplica.Store(int32(newReplica))
				} else {
					Logger.Errorf("[checkFixPoolReplicaLoop]invalid new replica: %v", newReplica)
				}
			}
		}
	}
}

// checked

// checked
func NewClusterManager(region string, isSnsEnabled bool, yamlConfig *YamlConfig) *ClusterManager {
	namespace := AutoScaleNamespace
	k8sConfig, K8sCli, MetricsCli, Cli := initK8sEnv(namespace)
	var snsManager *AwsSnsManager
	var err error
	if isSnsEnabled {
		snsManager, err = NewAwsSnsManager(region)
		if err != nil {
			panic(err)
		}
	}
	promCli, err := NewPromClientDefault()
	if err != nil {
		panic(err)
	}
	configManager := NewConfigManager(yamlConfig)
	ret := &ClusterManager{
		Namespace:     namespace,
		CloneSetName:  ReadNodeCloneSetName,
		SnsManager:    snsManager,
		PromClient:    promCli,
		ConfigManager: configManager,
		AutoScaleMeta: NewAutoScaleMeta(k8sConfig, configManager),
		tsContainer:   NewTimeSeriesContainer(),
		lstTsMap:      make(map[string]int64),

		K8sCli:                 K8sCli,
		MetricsCli:             MetricsCli,
		Cli:                    Cli,
		ExternalFixPoolReplica: atomic.Int32{},
	}
	ret.ExternalFixPoolReplica.Store(FixPoolDefaultReplica)
	ret.initK8sComponents()

	ret.initRangeMetricsFromPromethues(HardCodeMaxScaleIntervalSecOfCfg)

	go ret.collectMetricsFromPromethuesLoop()
	go ret.manageAnalyzeTasks()
	go ret.collectTaskCntMetricsFromPromethuesLoop()
	go ret.scanPodsStatesLoop()
	go ret.checkFixPoolReplicaLoop()

	return ret
}

// checked
// return is successful to handle
func (c *ClusterManager) handleCloneSetApiError(err error, caller string) bool {
	errStr := err.Error()
	Logger.Errorf("[error][%v]handleClonesetApiError, err: %+v", caller, errStr)
	// if strings.Contains(errStr, "please apply your changes to the latest version") {
	ret, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Get(context.TODO(), c.CloneSetName, metav1.GetOptions{})
	if err != nil {
		Logger.Errorf("[error][%v]handleClonesetApiError again, failed to get latest version of cloneset, err: %+v", caller, err.Error())
	} else {
		c.CloneSet = ret
		return true
	}
	// }
	return false
}

// checked
func (c *ClusterManager) addNewPods(delta int32, retryCnt int) (*v1alpha1.CloneSet, error) {
	c.muOfCloneSet.Lock()
	// defer c.muOfCloneSet.Unlock()
	// if delta <= 0 {
	// 	return cloneSet, fmt.Errorf("delta <= 0")
	// }
	// if int32(from) != *cloneSet.Spec.Replicas {
	// 	return cloneSet, fmt.Errorf("int32(from) != *cloneSet.Spec.Replicas")
	// }
	oldRelica := *c.CloneSet.Spec.Replicas
	newReplicas := new(int32)
	*newReplicas = int32(*c.CloneSet.Spec.Replicas + delta)
	c.CloneSet.Spec.Replicas = newReplicas
	ret, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Update(context.TODO(), c.CloneSet, metav1.UpdateOptions{})
	if err != nil {
		*c.CloneSet.Spec.Replicas = oldRelica
		Logger.Infof("[ClusterManager][addPods] failed, curReplica:%v newReplica:%v, error: %v", oldRelica, *newReplicas, err.Error())
		if c.handleCloneSetApiError(err, "ClusterManager.addNewPods") {
			if retryCnt > 0 {
				c.muOfCloneSet.Unlock()
				return c.addNewPods(delta, retryCnt-1)
			}
		}
		c.muOfCloneSet.Unlock()
		MetricOfClonesetReplicaAddFailedCnt.Add(float64(delta))
		return c.CloneSet, fmt.Errorf(err.Error())
	} else {
		c.CloneSet = ret.DeepCopy()
		c.muOfCloneSet.Unlock()
		Logger.Infof("[ClusterManager][addPods] addNewPods, curReplica:%v newReplica:%v", oldRelica, *newReplicas)
		MetricOfClonesetReplicaAddSuccessCnt.Add(float64(delta))
		return ret, nil
	}

}

// checked
func (c *ClusterManager) removePods(pods2del []string, retryCnt int) (*v1alpha1.CloneSet, error) {
	c.muOfCloneSet.Lock()
	// defer c.muOfCloneSet.Unlock()
	oldRelica := *c.CloneSet.Spec.Replicas
	newReplicas := new(int32)
	*newReplicas = int32(*c.CloneSet.Spec.Replicas - int32(len(pods2del)))
	c.CloneSet.Spec.Replicas = newReplicas
	c.CloneSet.Spec.ScaleStrategy.PodsToDelete = pods2del
	ret, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Update(context.TODO(), c.CloneSet, metav1.UpdateOptions{})
	if err != nil {
		*c.CloneSet.Spec.Replicas = oldRelica
		c.CloneSet.Spec.ScaleStrategy.PodsToDelete = make([]string, 0)
		// Logger.Errorf("[error][ClusterManager.addNewPods] error encountered! err:%v", err.Error())
		Logger.Errorf("[ClusterManager][removePods] failed, curReplica:%v newReplica:%v, error: %v", oldRelica, *newReplicas, err.Error())
		if c.handleCloneSetApiError(err, "ClusterManager.removePods") {
			if retryCnt > 0 {
				c.muOfCloneSet.Unlock()
				return c.removePods(pods2del, retryCnt-1)
			}
		}
		c.muOfCloneSet.Unlock()
		MetricOfClonesetReplicaDelFailedCnt.Add(float64(len(pods2del)))
		return c.CloneSet, fmt.Errorf(err.Error())
	} else {
		c.CloneSet = ret.DeepCopy()
		ret.Spec.ScaleStrategy.PodsToDelete = nil // reset field Spec.ScaleStrategy.PodsToDelete
		c.muOfCloneSet.Unlock()
		Logger.Infof("[ClusterManager][removePods] removePods, curReplica:%v newReplica:%v, pods2del: %+v", oldRelica, *newReplicas, pods2del)
		MetricOfClonesetReplicaDelSuccessCnt.Add(float64(len(pods2del)))
		return ret, nil
	}
}

func (c *ClusterManager) podPrepareLoop() {
	defer c.wg.Done()
	for {
		time.Sleep(1000 * time.Millisecond)
		if atomic.LoadInt32(&c.shutdown) != 0 {
			return
		}
		c.AutoScaleMeta.PrewarmPool.DoPodsWarm(c)
	}
}

func (c *ClusterManager) Wait() {
	c.wg.Wait()
}

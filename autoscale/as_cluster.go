package autoscale

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openkruise/kruise-api/apps/v1alpha1"
	kruiseclientset "github.com/openkruise/kruise-api/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
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

var OptionRunModeIsLocal = false
var EnvRegion string

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
	K8sCli        *kubernetes.Clientset
	MetricsCli    *metricsv.Clientset
	Cli           *kruiseclientset.Clientset
	CloneSet      *v1alpha1.CloneSet
	wg            sync.WaitGroup
	shutdown      int32 // atomic
	watchMu       sync.Mutex
	watcher       watch.Interface
	muOfCloneSet  sync.Mutex

	tsContainer *TimeSeriesContainer
	lstTsMap    map[string]int64
}

// cnt: want, create, get

// TODO expire of removed Pod in tsContainer,lstTsMap

func (c *ClusterManager) collectMetricsFromMetricServer() {
	defer c.wg.Done()
	as_meta := c.AutoScaleMeta
	lstTsMap := c.lstTsMap
	tsContainer := c.tsContainer
	hasNew := false
	for {
		time.Sleep(200 * time.Millisecond)
		if atomic.LoadInt32(&c.shutdown) != 0 {
			return
		}
		labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": c.CloneSetName}}
		// st := time.Now().UnixNano()
		podMetricsList, err := c.MetricsCli.MetricsV1beta1().PodMetricses(c.Namespace).List(
			context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
		if err != nil {
			panic(err.Error())
		}
		mint := int64(math.MaxInt64)
		maxt := int64(0)

		for _, pod := range podMetricsList.Items {
			lstTs, ok := lstTsMap[pod.Name]
			if !ok || pod.Timestamp.Unix() > lstTs {
				tsContainer.Insert(pod.Name, pod.Timestamp.Unix(),
					[]float64{
						pod.Containers[0].Usage.Cpu().AsApproximateFloat64(),
						pod.Containers[0].Usage.Memory().AsApproximateFloat64(),
					})
				lstTsMap[pod.Name] = pod.Timestamp.Unix()

				snapshot := tsContainer.GetSnapshotOfTimeSeries(pod.Name)

				hasNew = true
				mint = Min(snapshot.MinTime, mint)
				maxt = Max(snapshot.MaxTime, maxt)

			}

		}

		// just print tenant's avg metrics
		if hasNew {
			hasNew = false
			tArr := c.AutoScaleMeta.GetTenantNames()
			for _, tName := range tArr {
				stats, _ := as_meta.ComputeStatisticsOfTenant(tName, tsContainer, "collectMetrics")
				log.Printf("[collectMetrics]Tenant %v statistics: cpu: %v %v mem: %v %v time_range:%v~%v\n", tName,
					stats[0].Avg(),
					stats[0].Cnt(),
					stats[1].Avg(),
					stats[1].Cnt(),
					mint, maxt,
				)
			}
		}
	}

}

func (c *ClusterManager) collectMetricsFromPromethues() {
	defer c.wg.Done()
	as_meta := c.AutoScaleMeta
	lstTsMap := c.lstTsMap
	tsContainer := c.tsContainer
	hasNew := false
	///TODO init timeseries of pods via RangeQuery
	for {
		time.Sleep(200 * time.Millisecond)
		if atomic.LoadInt32(&c.shutdown) != 0 {
			return
		}
		log.Println("[info][collectMetricsFromPromethues] query cpu")
		metricOfPods, err := c.PromClient.QueryCpu()
		if err != nil {
			continue
		}
		// labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": c.CloneSetName}}
		// st := time.Now().UnixNano()
		// podMetricsList, err := c.MetricsCli.MetricsV1beta1().PodMetricses(c.Namespace).List(
		// context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
		// if err != nil {
		// panic(err.Error())
		// }
		mint := int64(math.MaxInt64)
		maxt := int64(0)

		for podName, metric := range metricOfPods {
			lstTs, ok := lstTsMap[podName]
			if !ok || metric.time > lstTs {
				tsContainer.Insert(podName, metric.time,
					[]float64{
						metric.value,
						0.0, //TODO remove this dummy mem metric
					})
				lstTsMap[podName] = metric.time

				snapshot := tsContainer.GetSnapshotOfTimeSeries(podName)

				hasNew = true
				mint = Min(snapshot.MinTime, mint)
				maxt = Max(snapshot.MaxTime, maxt)

			}

		}

		// just print tenant's avg metrics
		if hasNew {
			hasNew = false
			tArr := c.AutoScaleMeta.GetTenantNames()
			for _, tName := range tArr {
				stats, _ := as_meta.ComputeStatisticsOfTenant(tName, tsContainer, "collectMetrics")
				log.Printf("[collectMetrics]Tenant %v statistics: cpu: %v %v mem: %v %v time_range:%v~%v\n", tName,
					stats[0].Avg(),
					stats[0].Cnt(),
					stats[1].Avg(),
					stats[1].Cnt(),
					mint, maxt,
				)
			}
		}
	}

}

func (c *ClusterManager) analyzeMetrics() {
	// TODO implement

	// c.tsContainer.GetSnapshotOfTimeSeries()
	defer c.wg.Done()
	lastTs := int64(0)
	for {
		time.Sleep(100 * time.Millisecond)
		if atomic.LoadInt32(&c.shutdown) != 0 {
			return
		}
		tenants := c.AutoScaleMeta.GetTenants()
		for _, tenant := range tenants {
			if tenant.GetState() != TenantStateResumed { // tenant not available
				continue
			}
			tenant.TryToReloadConf()
			cntOfPods := tenant.GetCntOfPods()
			if cntOfPods < tenant.GetMinCntOfPod() {
				log.Printf("[analyzeMetrics] StateResume and cntOfPods < tenant.MinCntOfPo, add more pods, minCntOfPods:%v tenant: %v\n", tenant.GetMinCntOfPod(), tenant.Name)
				c.AutoScaleMeta.ResizePodsOfTenant(cntOfPods, tenant.conf.GetInitCntOfPod(), tenant.Name, c.tsContainer)
				if c.SnsManager != nil {
					c.SnsManager.TryToPublishTopology(tenant.Name, time.Now().UnixNano(), tenant.GetPodNames()) // public latest topology into SNS
				}
			} else {
				stats, podCpuMap := c.AutoScaleMeta.ComputeStatisticsOfTenant(tenant.Name, c.tsContainer, "analyzeMetrics")
				cpuusage := stats[0].Avg()

				//Mock Metrics
				// CoreOfPod := DefaultCoreOfPod

				// Print Debug Info
				curTs := time.Now().Unix()
				// cpuusage := MockComputeStatisticsOfTenant(CoreOfPod, cntOfPods, tenant.MaxCntOfPod)
				if lastTs != curTs {
					// log.Printf("[analyzeMetrics]ComputeStatisticsOfTenant, pods Of Tenant %v: %+v\n", tenant.Name, tenant.GetPodNames())
					log.Printf("[analyzeMetrics]ComputeStatisticsOfTenant, Tenant %v , cpu usage: %v %v , PodsCpuMap: %+v \n", tenant.Name,
						stats[0].Avg(), stats[0].Cnt(), podCpuMap)
					// log.Printf("[ComputeStatisticsOfTenant] cpu usage: %v\n", cpuusage)
					lastTs = curTs
				}
				minCpuUsageThreshold, maxCpuUsageThreshold := tenant.conf.GetLowerAndUpperCpuScaleThreshold()
				bestPods, _ := ComputeBestPodsInRuleOfCompute(tenant, cpuusage, minCpuUsageThreshold, maxCpuUsageThreshold)
				if bestPods != -1 && cntOfPods != bestPods {
					log.Printf("[analyzeMetrics] resize pods, from %v to  %v , tenant: %v\n", tenant.GetCntOfPods(), bestPods, tenant.Name)
					c.AutoScaleMeta.ResizePodsOfTenant(cntOfPods, bestPods, tenant.Name, c.tsContainer)
					if c.SnsManager != nil {
						c.SnsManager.TryToPublishTopology(tenant.Name, time.Now().UnixNano(), tenant.GetPodNames()) // public latest topology into SNS
					}
				} else {
					// log.Printf("[analyzeMetrics] pods unchanged cnt:%v, bestCnt:%v, tenant:%v \n", tenant.GetCntOfPods(), bestPods, tenant.Name)
				}
			}
			// tenant.IntoTenant()
		}
	}

}

func Int32Ptr(val int32) *int32 {
	ret := new(int32)
	*ret = int32(val)
	return &val
}

func (c *ClusterManager) Shutdown() {
	log.Println("[ClusterManager]Shutdown")
	atomic.StoreInt32(&c.shutdown, 1)
	c.watchMu.Lock()
	c.watcher.Stop()
	c.watchMu.Unlock()
	c.wg.Wait()
}

func (c *ClusterManager) Pause(tenant string) bool {
	return c.AutoScaleMeta.Pause(tenant)
}

func (c *ClusterManager) Resume(tenant string) bool {
	return c.AutoScaleMeta.Resume(tenant, c.tsContainer)
}

func (c *ClusterManager) watchPodsLoop(resourceVersion string) {
	defer c.wg.Done()
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
			panic(err.Error())
		}

		c.watchMu.Lock()
		c.watcher = watcher
		c.watchMu.Unlock()

		ch := watcher.ResultChan()

		// LISTEN TO CHANNEL
		for {
			e, more := <-ch
			if !more {
				log.Printf("watchPods channel closed\n")
				break
			}
			pod, ok := e.Object.(*v1.Pod)
			if !ok {
				continue
			}
			resourceVersion = pod.ResourceVersion
			switch e.Type {
			case watch.Added:
				c.AutoScaleMeta.UpdatePod(pod)
			case watch.Modified:
				c.AutoScaleMeta.UpdatePod(pod)
			case watch.Deleted:
				c.AutoScaleMeta.HandleK8sDelPodEvent(pod)
			default:
				fallthrough
			case watch.Error, watch.Bookmark: //TODO handle it
				continue
			}
			// log.Printf("act,ns,name,phase,reason,ip,noOfContainer: %v %v %v %v %v %v %v\n", e.Type,
			// 	pod.Namespace,
			// 	pod.Name,
			// 	pod.Status.Phase,
			// 	pod.Status.Reason,
			// 	pod.Status.PodIP,
			// 	len(pod.Status.ContainerStatuses))

		}
	}

}

// func (c *ClusterManager) scanStateOfPods() {
// 	c.AutoScaleMeta.scanStateOfPods()
// }

// ignore error
func (c *ClusterManager) loadPods() string {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": c.CloneSetName}}
	pods, err := c.K8sCli.CoreV1().Pods(c.Namespace).List(context.TODO(),
		metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return ""
	}
	resVer := pods.ListMeta.ResourceVersion
	for _, pod := range pods.Items {
		// //!!!TEST BEGIN
		// playLoadBytes := `
		// {
		// 	"metadata": {
		// 		"labels": {
		// 			"key1" : "value1",
		// 			"key2" : "value2"
		// 		}
		// 	}
		// }
		// `
		// c.K8sCli.CoreV1().Pods(c.Namespace).Patch(context.TODO(), pod.Name, k8stypes.StrategicMergePatchType, []byte(playLoadBytes), metav1.PatchOptions{})
		// //!!!TEST END
		c.AutoScaleMeta.UpdatePod(&pod)
	}
	return resVer
}

func (c *ClusterManager) getComputePodAntiAffinity() *v1.PodAntiAffinity {
	if OptionRunModeIsLocal {
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

// TODO load existed pods
func (c *ClusterManager) initK8sComponents() {
	// create cloneset if not exist
	cloneSetList, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	log.Printf("list clonneSet: %+v \n", len(cloneSetList.Items))
	found := false
	for _, cloneSet := range cloneSetList.Items {
		if cloneSet.Name == c.CloneSetName {
			found = true
			break
		}
	}
	var retCloneset *v1alpha1.CloneSet
	if !found {
		// volumeName := "tiflash-readnode-data-vol"
		/// TODO ensure one pod one node and fixed nodegroup
		//create cloneSet since there is no desired cloneSet
		cloneSet := v1alpha1.CloneSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: c.CloneSetName,
				Labels: map[string]string{
					"app": c.CloneSetName,
				}},
			Spec: v1alpha1.CloneSetSpec{
				Replicas: Int32Ptr(int32(c.AutoScaleMeta.SoftLimit)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": c.CloneSetName,
					}},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": c.CloneSetName,
						},
					},
					// pod anti affinity
					Spec: v1.PodSpec{
						NodeSelector: map[string]string{
							"tiflash.used-for-compute": "true",
							// "node.kubernetes.io/instance-type": "m6a.2xlarge", // TODO use a non-hack way to bind readnode pod to specific nodes
						},
						Affinity: &v1.Affinity{
							PodAntiAffinity: c.getComputePodAntiAffinity(),
						},
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
								},
								Name:            "supervisor",
								Image:           "bestwoody/supervisor:1",
								ImagePullPolicy: "Always",
								// VolumeMounts: []v1.VolumeMount{
								// 	{
								// 		Name:      volumeName,
								// 		MountPath: "/usr/share/nginx/html",
								// 	}},
							},
						},
					},
				},
				// VolumeClaimTemplates: []v1.PersistentVolumeClaim{
				// 	{
				// 		ObjectMeta: metav1.ObjectMeta{
				// 			Name: volumeName,
				// 		},
				// 		Spec: v1.PersistentVolumeClaimSpec{
				// 			AccessModes: []v1.PersistentVolumeAccessMode{
				// 				"ReadWriteOnce",
				// 			},
				// 			Resources: v1.ResourceRequirements{
				// 				Requests: v1.ResourceList{
				// 					"storage": resource.MustParse("20Gi"),
				// 				},
				// 			},
				// 		},
				// 	},
				// },
			},
		}
		log.Println("create clonneSet")
		c.AutoScaleMeta.PrewarmPool.cntOfPending.Add(*cloneSet.Spec.Replicas)
		retCloneset, err = c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Create(context.TODO(), &cloneSet, metav1.CreateOptions{})

	} else {
		log.Println("get clonneSet")
		retCloneset, err = c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Get(context.TODO(), c.CloneSetName, metav1.GetOptions{})
	}
	if err != nil {
		panic(err.Error())
	} else {
		c.CloneSet = retCloneset.DeepCopy()
	}

	// load k8s pods of cloneset
	resVer := c.loadPods()

	// TODO periodically call ScanStateOfPods()
	c.AutoScaleMeta.ScanStateOfPods()

	// watch changes of pods
	c.wg.Add(2)

	go c.watchPodsLoop(resVer)

	// pod prepare & GC
	go c.podPrepareLoop()
}

//
// func (c *ClusterManager) recoverStatesOfPods() {
// 	log.Println("[ClusterManager] recoverStatesOfPods(): unimplement")
// 	// c.AutoScaleMeta.recoverStatesOfPods()
// }

func playground(k8sCli *kubernetes.Clientset) {
	// k8sCli.CoreV1().Namespaces("tiflash-autoscale").Patch()
}

func initK8sEnv(Namespace string) (config *restclient.Config, K8sCli *kubernetes.Clientset, MetricsCli *metricsv.Clientset, Cli *kruiseclientset.Clientset) {
	config, err := getK8sConfig()
	if err != nil {
		panic(err.Error())
	}
	MetricsCli, err = metricsv.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	K8sCli, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	Cli = kruiseclientset.NewForConfigOrDie(config)

	// create NameSpace if not exsist
	_, err = K8sCli.CoreV1().Namespaces().Get(context.TODO(), Namespace, metav1.GetOptions{})
	if err != nil {
		_, err = K8sCli.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: Namespace,
				Labels: map[string]string{
					"ns": Namespace,
				}}}, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}
	}
	return config, K8sCli, MetricsCli, Cli
}

// podstat:   init---->prewarmed<--->ComputePod

func NewClusterManager(region string, isSnsEnabled bool) *ClusterManager {
	namespace := "tiflash-autoscale"
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
	ret := &ClusterManager{
		Namespace:     namespace,
		CloneSetName:  "readnode",
		SnsManager:    snsManager,
		PromClient:    promCli,
		AutoScaleMeta: NewAutoScaleMeta(k8sConfig),
		tsContainer:   NewTimeSeriesContainer(DefaultCapOfSeries, promCli),
		lstTsMap:      make(map[string]int64),

		K8sCli:     K8sCli,
		MetricsCli: MetricsCli,
		Cli:        Cli,
	}
	ret.initK8sComponents()

	ret.wg.Add(2)
	go ret.collectMetricsFromMetricServer()
	go ret.analyzeMetrics()
	return ret
}

// return is successful to handle
func (c *ClusterManager) handleCloneSetApiError(err error, caller string) bool {
	errStr := err.Error()
	log.Printf("[error][%v]handleClonesetApiError, err: %+v\n", caller, errStr)
	if strings.Contains(errStr, "please apply your changes to the latest version") {
		ret, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Get(context.TODO(), c.CloneSetName, metav1.GetOptions{})
		if err != nil {
			log.Printf("[error][%v]handleClonesetApiError, failed to get latest version of cloneset, err: %+v\n", caller, err.Error())
		} else {
			c.CloneSet = ret
			return true
		}
	}
	return false
}

func (c *ClusterManager) addNewPods(delta int32, retryCnt int) (*v1alpha1.CloneSet, error) {
	c.muOfCloneSet.Lock()
	defer c.muOfCloneSet.Unlock()
	// if delta <= 0 {
	// 	return cloneSet, fmt.Errorf("delta <= 0")
	// }
	// if int32(from) != *cloneSet.Spec.Replicas {
	// 	return cloneSet, fmt.Errorf("int32(from) != *cloneSet.Spec.Replicas")
	// }
	newReplicas := new(int32)
	*newReplicas = int32(*c.CloneSet.Spec.Replicas + delta)
	c.CloneSet.Spec.Replicas = newReplicas
	ret, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Update(context.TODO(), c.CloneSet, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("[ClusterManager.addPods] failed, error: %v\n", err.Error())
		if c.handleCloneSetApiError(err, "ClusterManager.addNewPods") {
			if retryCnt > 0 {
				return c.addNewPods(delta, retryCnt-1)
			}
		}
		return c.CloneSet, fmt.Errorf(err.Error())
	} else {
		c.CloneSet = ret.DeepCopy()
		return ret, nil
	}

}

func (c *ClusterManager) removePods(pods2del []string, retryCnt int) (*v1alpha1.CloneSet, error) {
	c.muOfCloneSet.Lock()
	defer c.muOfCloneSet.Unlock()
	newReplicas := new(int32)
	*newReplicas = int32(*c.CloneSet.Spec.Replicas - int32(len(pods2del)))
	c.CloneSet.Spec.Replicas = newReplicas
	c.CloneSet.Spec.ScaleStrategy.PodsToDelete = pods2del
	ret, err := c.Cli.AppsV1alpha1().CloneSets(c.Namespace).Update(context.TODO(), c.CloneSet, metav1.UpdateOptions{})
	if err != nil {
		// log.Printf("[error][ClusterManager.addNewPods] error encountered! err:%v\n", err.Error())
		log.Printf("[ClusterManager.removePods] failed, error: %v\n", err.Error())
		if c.handleCloneSetApiError(err, "ClusterManager.removePods") {
			if retryCnt > 0 {
				return c.removePods(pods2del, retryCnt-1)
			}
		}
		return c.CloneSet, fmt.Errorf(err.Error())
	} else {
		c.CloneSet = ret.DeepCopy()
		ret.Spec.ScaleStrategy.PodsToDelete = nil // reset field Spec.ScaleStrategy.PodsToDelete
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
		// TODO  addNewPods / removePods

	}
}

// func (c *ClusterManager) AddNewPods(from int, delta int) (*v1alpha1.CloneSet, error) {
// 	return AddNewPods(c, c.Cli, c.Namespace, c.CloneSet, from, delta)
// }

func (c *ClusterManager) Wait() {
	c.wg.Wait()
}

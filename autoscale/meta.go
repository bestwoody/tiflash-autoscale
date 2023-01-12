package autoscale

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	supervisor "github.com/tikv/pd/supervisor_proto"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

const (
	// PodStateUnassigned = 0
	// PodStateAssigned   = 1
	// PodStateInit       = 2
	// PodStateUnknown     = 3
	TenantStateResumed  = 0
	TenantStateResuming = 1
	TenantStatePaused   = 2
	TenantStatePausing  = 3
	TenantStateUnknown  = 4

	FailCntCheckTimeWindow = 300 // 300s used for DoPodsPreWarm
)

var (
	DefaultMinCntOfPod        = 1
	DefaultMaxCntOfPod        = 4
	DefaultCoreOfPod          = 8
	DefaultLowerLimit         = 0.2
	DefaultUpperLimit         = 0.8
	DefaultPrewarmPoolCap     = 4
	CapacityOfStaticsAvgSigma = 6
	// DefaultCapOfSeries        = 6  ///default scale interval: 1min. 6 * MetricResolutionSeconds(10s) = 60s (1min)
	MetricResolutionSeconds = 10 // metric step: 10s

	DefaultAutoPauseIntervalSeconds  = 60
	DefaultScaleIntervalSeconds      = 60
	HardCodeMaxScaleIntervalSecOfCfg = 3600
)

var PrewarmPoolCap = DefaultPrewarmPoolCap

func TenantState2String(state int32) string {
	if state == TenantStateResumed {
		return TenantStateResumedString
	} else if state == TenantStateResuming {
		return TenantStateResumingString
	} else if state == TenantStatePaused {
		return TenantStatePausedString
	} else if state == TenantStateUnknown {
		return TenantStateUnknownString
	}
	return TenantStatePausingString
}

type PodDesc struct {
	Name string
	IP   string
	// State int32 // 0: unassigned 1:assigned

	TenantName        string
	StartTimeOfAssign int64        //startTime of tenant's assignment
	mu                sync.RWMutex /// TODO use it //TODO add pod level lock!!!

	muOfGrpc        sync.Mutex
	isStateChanging atomic.Bool
	// pod        *v1.Pod
}

// checked
func (p *PodDesc) SetTenantInfo(tenantName string, startTimeOfAssgin int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TenantName = tenantName
	p.StartTimeOfAssign = startTimeOfAssgin
}

// checked
func (p *PodDesc) ClearTenantInfo() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TenantName = ""
	p.StartTimeOfAssign = time.Now().Unix()
}

// checked
func (p *PodDesc) GetTenantInfo() (string, int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.TenantName, p.StartTimeOfAssign
}

// checked
func (c *PodDesc) AssignTenantWithMockConf(tenant string) (resp *supervisor.Result, err error) {
	c.muOfGrpc.Lock()
	defer c.muOfGrpc.Unlock()
	return AssignTenantHardCodeArgs(c.IP, tenant)
}

// checked
func (c *PodDesc) UnassignTenantWithMockConf(tenant string, forceShutdown bool) (resp *supervisor.Result, err error) {
	c.muOfGrpc.Lock()
	defer c.muOfGrpc.Unlock()
	return UnassignTenant(c.IP, tenant, forceShutdown)
}

// checked
func (podDesc *PodDesc) ApiGetCurrentTenantAndCorrect(meta *AutoScaleMeta, atStartup bool) (*supervisor.GetTenantResponse, error) {
	if podDesc.muOfGrpc.TryLock() {
		defer podDesc.muOfGrpc.Unlock()
		if podDesc.isStateChanging.Load() {
			Logger.Warnf("[PodDesc][GetCurrentTenant]state is changing, skip.")
			return nil, fmt.Errorf("state is changing")
		}
		resp, err := GetCurrentTenant(podDesc.IP)
		Logger.Debugf("[PodDesc][GetCurrentTenant] result tenant:%v pod:%v resp:%v", podDesc.TenantName, podDesc.Name, resp.String())
		if err != nil {
			Logger.Errorf("[PodDesc][GetCurrentTenant]failed to GetCurrentTenant, podname: %v ip: %v, error: %v", podDesc.Name, podDesc.IP, err.Error())
		} else {
			// if resp.GetTenantID() != "" {
			oldTenant, _ := podDesc.GetTenantInfo()
			if (atStartup || oldTenant != "" /*startup any pod or runtime tenant's pod */) && !resp.IsUnassigning && (oldTenant != resp.GetTenantID() || podDesc.StartTimeOfAssign != resp.StartTime) {
				Logger.Infof("[PodDesc][GetCurrentTenant]state need to update, podname: %v tenantDiff:[%v vs %v] stimeDiff:[%v vs %v]", podDesc.Name, podDesc.TenantName, resp.GetTenantID(), podDesc.StartTimeOfAssign, resp.StartTime)
				meta.mu.Lock()
				meta.updateLocalMetaPodOfTenant(podDesc.Name, podDesc, resp.GetTenantID(), resp.StartTime)
				meta.mu.Unlock()
			}
			// }
		}
		return resp, err
	} else {
		Logger.Errorf("[PodDesc][GetCurrentTenant]trylock failed, pod:%v", podDesc.Name)
		return nil, fmt.Errorf("trylock failed")
	}
}

type TenantConf struct { // TODO use it
	tidbStatusAddr string
	pdAddr         string
}

type TenantDesc struct {
	Name     string
	podMap   map[string]*PodDesc
	podList  []*PodDesc
	State    int32
	mu       sync.RWMutex
	ResizeMu sync.Mutex

	conf            ConfigOfComputeCluster        /// TODO copy from configManager, reload for each analyze loop
	refOfLatestConf *ConfigOfComputeClusterHolder /// TODO assign it // DO NOT directly read it ,since it is cocurrently being writed by other thread
	// conf        TenantConf // TODO use it
}

func (c *TenantDesc) Dump() string {
	pods := c.GetPodNames()
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("TenantDesc{name:%v, pods:%+v, state:%v, conf:%v}", c.Name, pods, TenantState2String(c.State), c.conf.Dump())
}

// checked
func (c *TenantDesc) SortPodsAtStartUp() {
	c.mu.Lock()
	defer c.mu.Unlock()
	sort.Slice(c.podList, func(i, j int) bool {
		return c.podList[i].StartTimeOfAssign < c.podList[j].StartTimeOfAssign
	})
	if len(c.podList) > 0 {
		Logger.Infof("[TenantDesc][%v][SortPodsOnStartUp]first.startTime:%v last.startTime:%v", c.Name, c.podList[0].StartTimeOfAssign, c.podList[len(c.podList)-1].StartTimeOfAssign)
	}
}

// checked
func (c *TenantDesc) TryToReloadConf(forceUpdate bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.refOfLatestConf == nil {
		return false
	}
	if forceUpdate || c.refOfLatestConf.HasChanged(c.conf.LastModifiedTs) {
		c.conf = c.refOfLatestConf.DeepCopy()
		if (c.conf.MinCores%DefaultCoreOfPod != 0) || (c.conf.MaxCores%DefaultCoreOfPod != 0) {
			Logger.Errorf("min/max cores not completedly divided by DefaultCoreOfPod, TidbCluster: %v , minCores: %v , maxCore: %v , coresOfPod:%v \n",
				c.conf.ConfigOfTiDBCluster.Name, c.conf.MinCores, c.conf.MaxCores, DefaultCoreOfPod)
		}
		// c.MinCntOfPod = c.conf.MinCores / DefaultCoreOfPod
		// c.MaxCntOfPod = c.conf.MaxCores / DefaultCoreOfPod
		return true
	}
	return false
}

// checked
func (c *TenantDesc) GetInitCntOfPod() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conf.GetInitCntOfPod()
}

// checked
func (c *TenantDesc) GetLowerAndUpperCpuScaleThreshold() (float64, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conf.GetLowerAndUpperCpuScaleThreshold()
}

// checked
func (c *TenantDesc) GetMinCntOfPod() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ret := MaxInt(c.conf.MinCores/DefaultCoreOfPod, 1)
	if c.conf.MinCores%DefaultCoreOfPod != 0 || c.conf.MinCores <= 0 {
		Logger.Errorf("[Conf][%v]invalid min_cores:%v, min_pods:%v", c.Name, c.conf.MinCores, ret)
	}
	return ret
}

// checked
func (c *TenantDesc) GetMaxCntOfPod() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ret := MaxInt(c.conf.MaxCores/DefaultCoreOfPod, MaxInt(c.conf.MinCores/DefaultCoreOfPod, 1))
	if c.conf.MaxCores%DefaultCoreOfPod != 0 || c.conf.MaxCores <= 0 {
		Logger.Errorf("[Conf][%v]invalid max_cores:%v, max_pods:%v", c.Name, c.conf.MaxCores, ret)
	}
	return ret
}

// checked
func (c *TenantDesc) IsDisabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conf.Disabled
}

// checked
func (c *TenantDesc) GetScaleIntervalSec() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conf.WindowSeconds
}

// checked
func (c *TenantDesc) GetAutoPauseIntervalSec() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conf.AutoPauseIntervalSeconds
}

// checked
func (c *TenantDesc) GetCntOfPods() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.podMap)
}

// checked, it has TODO
func (c *TenantDesc) SetPod(k string, v *PodDesc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.podMap[k]
	if !ok {
		c.podList = append(c.podList, v)
	}

	c.podMap[k] = v
	v.TenantName = c.Name ///TODO use poddesc.mutex
}

// checked
func (c *TenantDesc) SetPodWithTenantInfo(k string, v *PodDesc, startTimeOfAssign int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.podMap[k]
	if !ok {
		c.podList = append(c.podList, v)
	}

	c.podMap[k] = v
	v.SetTenantInfo(c.Name, startTimeOfAssign)
}

// checked
func (c *TenantDesc) GetPod(k string) (*PodDesc, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.podMap[k]
	return v, ok
}

// checked
func (c *TenantDesc) RemovePod(k string) *PodDesc {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.podMap[k]
	if ok {
		newArr := make([]*PodDesc, 0, len(c.podList))
		for _, item := range c.podList {
			if v != item {
				newArr = append(newArr, item)
			}
		}
		c.podList = newArr
		delete(c.podMap, k)
		v.ClearTenantInfo()
		return v
	} else {
		return nil
	}
}

// checked
func (c *TenantDesc) popOnePod() *PodDesc {
	if len(c.podList) > 0 {
		curPod := c.podList[len(c.podList)-1]
		c.podList = c.podList[:len(c.podList)-1]
		_, ok := c.podMap[curPod.Name]
		if ok {
			delete(c.podMap, curPod.Name)
		}
		curPod.ClearTenantInfo()
		return curPod
	} else {
		return nil
	}
}

// checked
func (c *TenantDesc) PopPods(cnt int, ret []*PodDesc) (int, []*PodDesc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for cnt > 0 {
		v := c.popOnePod()
		if v != nil {
			ret = append(ret, v)
			cnt--
		} else {
			break
		}
	}
	return cnt, ret
}

// checked
func (c *TenantDesc) GetPodNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ret := make([]string, 0, len(c.podMap))
	for _, v := range c.podList {
		ret = append(ret, v.Name)
	}
	return ret
}

// checked
func (c *TenantDesc) GetPodAddrs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ret := make([]string, 0, len(c.podMap))
	for _, v := range c.podList {
		if v.IP != "" {
			ret = append(ret, fmt.Sprintf("%v:3930", v.IP))
		} else {
			Logger.Errorf("[TenantDesc][GetPodIps]pod ip is null! tenant:%v pod:%v", c.Name, v.Name)
		}
	}
	return ret
}

// checked
func (c *TenantDesc) switchState(from int32, to int32) bool {
	return atomic.CompareAndSwapInt32(&c.State, from, to)
}

// checked
func (c *TenantDesc) SyncStatePausing() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.switchState(TenantStateResumed, TenantStatePausing)
}

// checked
func (c *TenantDesc) SyncStatePaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.switchState(TenantStatePausing, TenantStatePaused)
}

// checked
func (c *TenantDesc) SyncStateResuming() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.switchState(TenantStatePaused, TenantStateResuming)
}

// checked
func (c *TenantDesc) SyncStateResumed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.switchState(TenantStateResuming, TenantStateResumed)
}

// checked
func (c *TenantDesc) SetState(state int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	atomic.StoreInt32(&c.State, state)
}

// checked
func (c *TenantDesc) IsStateCorrect() (bool, int32) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if atomic.LoadInt32(&c.State) == TenantStatePaused && len(c.podMap) > 0 {
		return false, TenantStateResumed
	}
	return true, atomic.LoadInt32(&c.State)
}

// checked
func (c *TenantDesc) GetState() int32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return atomic.LoadInt32(&c.State)
}

// func NewTenantDescDefault(name string) *TenantDesc {
// 	return NewTenantDesc(name, DefaultMinCntOfPod, DefaultMaxCntOfPod)
// }

// func NewTenantDesc(name string, minPods int, maxPods int) *TenantDesc {
// 	return NewTenantDescWithState(name, minPods, maxPods, TenantStateResumed)
// }

func NewAutoPauseTenantDescWithState(name string, minPods int, maxPods int, state int32) *TenantDesc {
	return &TenantDesc{
		Name:  name,
		State: state,
		// MinCntOfPod: minPods,
		// MaxCntOfPod: maxPods,
		podMap:  make(map[string]*PodDesc),
		podList: make([]*PodDesc, 0, 64),
		conf: ConfigOfComputeCluster{
			Disabled:                 false,                           ///TODO  disable or not defualt?
			AutoPauseIntervalSeconds: DefaultAutoPauseIntervalSeconds, // 5min defualt
			MinCores:                 minPods * DefaultCoreOfPod,
			MaxCores:                 maxPods * DefaultCoreOfPod,
			InitCores:                minPods * DefaultCoreOfPod,
			WindowSeconds:            DefaultScaleIntervalSeconds,
			CpuScaleRules:            nil,
			ConfigOfTiDBCluster: &ConfigOfTiDBCluster{ // triger when modified: instantly reload compute pod's config  TODO handle version change case
				Name: name,
			},
			LastModifiedTs: 0,
		},
	}
}

func NewTenantDescWithConfigAndState(name string, confHolder *ConfigOfComputeClusterHolder, state int32) *TenantDesc {
	ret := NewTenantDescWithConfig(name, confHolder)
	ret.State = state
	return ret
}

func NewTenantDescWithConfig(name string, confHolder *ConfigOfComputeClusterHolder) *TenantDesc {
	ret := &TenantDesc{
		Name:            name,
		podMap:          make(map[string]*PodDesc),
		podList:         make([]*PodDesc, 0, 64),
		refOfLatestConf: confHolder,
	}
	ret.TryToReloadConf(true)
	return ret
}

type PrewarmPoolOpResult struct {
	failCnt int
	lastTs  int64
}

type PrewarmPool struct {
	mu         sync.Mutex
	WarmedPods *TenantDesc

	cntOfPending          atomic.Int32
	tenantLastOpResultMap map[string]*PrewarmPoolOpResult
	SoftLimit             int // expected size of pool
}

func NewPrewarmPool(warmedPods *TenantDesc) *PrewarmPool {
	return &PrewarmPool{
		WarmedPods:            warmedPods,
		cntOfPending:          atomic.Int32{},
		tenantLastOpResultMap: make(map[string]*PrewarmPoolOpResult),
		SoftLimit:             warmedPods.GetMaxCntOfPod(),
	}
}

// checked
func (p *PrewarmPool) DoPodsWarm(c *ClusterManager) {
	failCntTotal := 0
	p.mu.Lock()

	now := time.Now().Unix()
	for k, v := range p.tenantLastOpResultMap {
		if v.lastTs > now-FailCntCheckTimeWindow {
			failCntTotal += v.failCnt
		} else {
			delete(p.tenantLastOpResultMap, k)
		}
	}

	/// DO real pods resize!!!!
	delta := failCntTotal + p.SoftLimit - (int(p.cntOfPending.Load()) + p.WarmedPods.GetCntOfPods())

	if delta != 0 {
		Logger.Infof("[PrewarmPool]DoPodsWarm. failcnt:%v , delta:%v, pending: %v valid:%v ", failCntTotal, delta, p.cntOfPending.Load(), p.WarmedPods.GetCntOfPods())
	}
	p.mu.Unlock()

	// var ret *v1alpha1.CloneSet
	var err error
	if delta > 0 {
		p.cntOfPending.Add(int32(delta))
		Logger.Debugf("[CntOfPending]DoPodsWarm, add delta %v, result:%v", delta, p.cntOfPending.Load())
		_, err = c.addNewPods(int32(delta), 2)
		if err != nil { // revert
			p.cntOfPending.Add(int32(-delta))
			Logger.Debugf("[CntOfPending]DoPodsWarm, revert delta %v, result:%v", delta, p.cntOfPending.Load())
		}
	} else if delta < 0 {
		overCnt := p.WarmedPods.GetCntOfPods() - p.SoftLimit
		if overCnt > 0 {
			removeCnt := MinInt(-delta, overCnt)

			podsToDel, _ := p.getWarmedPods("", removeCnt)
			podNames := make([]string, 0, removeCnt)
			for _, v := range podsToDel {
				podNames = append(podNames, v.Name)
			}
			_, err = c.removePods(podNames, 2)
			if err != nil { // revert
				for _, pod := range podsToDel {
					p.putWarmedPod("", pod, false)
				}
			}
		}
	}
	if err != nil {
		Logger.Errorf("[error][PrewarmPool.DoPodsWarm] error encountered! err:%v", err.Error())
	}
}

// checked
func (p *PrewarmPool) getWarmedPods(tenantName string, cnt int) ([]*PodDesc, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	podnames := p.WarmedPods.GetPodNames()
	podsToAssign := make([]*PodDesc, 0, cnt)
	for _, k := range podnames {
		if cnt > 0 {
			v := p.WarmedPods.RemovePod(k)
			if v != nil {
				podsToAssign = append(podsToAssign, v)
				cnt--
			} else {
				Logger.Warnf("[PrewarmPool::getWarmedPods] p.WarmedPods.RemovePod fail, return nil!")
			}
		} else {
			//enough pods, break early
			break
		}
	}
	if tenantName != "" {
		p.tenantLastOpResultMap[tenantName] = &PrewarmPoolOpResult{
			failCnt: cnt,
			lastTs:  time.Now().Unix(),
		}
	}
	return podsToAssign, cnt
}

// checked
func (p *PrewarmPool) putWarmedPod(tenantName string, pod *PodDesc, isNewPod bool) {
	Logger.Infof("[PrewarmPool]put warmed pod tenant: %v pod: %v newPod:%v", tenantName, pod.Name, isNewPod)
	p.mu.Lock()
	defer p.mu.Unlock()
	if isNewPod {
		if p.cntOfPending.Load() > 0 { /// .it maybe <0 when startUp, it's used for that case but harmless since it will be correct after startUp. TODO use a more graceful way
			p.cntOfPending.Add(-1)
			Logger.Debugf("[CntOfPending]putWarmedPod result:%v", p.cntOfPending.Load())
		} else {
			Logger.Debugf("[CntOfPending]putWarmedPod, cntOfPending <= 0, cntOfPending:%v", p.cntOfPending.Load())
		}
	}
	p.WarmedPods.SetPod(pod.Name, pod) // no need to set startTimeOfAssign, since there is no tenantInfo
	if tenantName != "" {              // reset tenant's LastOpResult， since tenant returns pod back to pool, he has enough pods.
		p.tenantLastOpResultMap[tenantName] = &PrewarmPoolOpResult{
			failCnt: 0,
			lastTs:  time.Now().Unix(),
		}
	}
}

type AutoScaleMeta struct {
	mu         sync.Mutex //TODO use RwMutex
	tenantMap  map[string]*TenantDesc
	PodDescMap map[string]*PodDesc
	*PrewarmPool

	k8sCli *kubernetes.Clientset
	// configMap      *v1.ConfigMap //TODO expire entry of removed pod
	// cmMutex        sync.Mutex
	IsRuntimeReady atomic.Bool
}

// checked
func NewAutoScaleMeta(config *restclient.Config) *AutoScaleMeta {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	ret := &AutoScaleMeta{
		// Pod2tenant: make(map[string]string),
		tenantMap:   make(map[string]*TenantDesc),
		PodDescMap:  make(map[string]*PodDesc),
		PrewarmPool: NewPrewarmPool(NewAutoPauseTenantDescWithState("", 0, PrewarmPoolCap, TenantStateResumed)),
		k8sCli:      client,
	}
	if OptionRunMode == RunModeLocal {
		ret.loadTenants4Test()
	}
	// ret.initConfigMap()
	return ret
}

// checked
func (c *AutoScaleMeta) Dump() string {
	pod2ip := make(map[string]string)
	tenant2PodCntMap := make(map[string]([]string))
	c.mu.Lock()
	for k, v := range c.tenantMap {
		tenant2PodCntMap[k] = v.GetPodNames()
	}
	for k, v := range c.PodDescMap {
		pod2ip[k] = v.IP
	}
	// pendingCnt := c.pendingCnt
	c.mu.Unlock()
	return fmt.Sprintf("tenantcnt:%v, podcnt:%v, warmpool:%v tenants:{%+v}, pods:{%+v} ", len(tenant2PodCntMap), len(pod2ip), c.WarmedPods.GetPodNames(), tenant2PodCntMap, pod2ip)
}

// checked
func (c *AutoScaleMeta) setupManualPauseMockTenant(name string, minPods, maxPods int, disabled bool, WindowSeconds int, cpuScaleRule *CustomScaleRule) error {
	c.SetupTenantWithConfig(name, &ConfigOfComputeClusterHolder{
		Config: ConfigOfComputeCluster{
			Disabled:                 disabled, ///TODO  disable or not defualt?
			AutoPauseIntervalSeconds: 0,        // 0 means ManualPause
			MinCores:                 minPods * DefaultCoreOfPod,
			MaxCores:                 maxPods * DefaultCoreOfPod,
			InitCores:                minPods * DefaultCoreOfPod,
			WindowSeconds:            WindowSeconds,
			CpuScaleRules:            cpuScaleRule,
			ConfigOfTiDBCluster: &ConfigOfTiDBCluster{ // triger when modified: instantly reload compute pod's config  TODO handle version change case
				Name: name,
			},
			LastModifiedTs: time.Now().UnixNano(),
		},
	}, TenantStateResumed)
	return nil
}

// checked
func (c *AutoScaleMeta) setupAutoPauseMockTenant(name string, minPods, maxPods int, disabled bool, autoPauseIntervalSeconds int, WindowSeconds int, cpuScaleRule *CustomScaleRule, state int32) error {
	if autoPauseIntervalSeconds <= 0 {
		return fmt.Errorf("autoPauseIntervalSeconds <= 0")
	}
	c.SetupTenantWithConfig(name, &ConfigOfComputeClusterHolder{
		Config: ConfigOfComputeCluster{
			Disabled:                 disabled,                 ///TODO  disable or not defualt?
			AutoPauseIntervalSeconds: autoPauseIntervalSeconds, // 5min defualt
			MinCores:                 minPods * DefaultCoreOfPod,
			MaxCores:                 maxPods * DefaultCoreOfPod,
			InitCores:                minPods * DefaultCoreOfPod,
			WindowSeconds:            WindowSeconds,
			CpuScaleRules:            cpuScaleRule,
			ConfigOfTiDBCluster: &ConfigOfTiDBCluster{ // triger when modified: instantly reload compute pod's config  TODO handle version change case
				Name: name,
			},
			LastModifiedTs: time.Now().UnixNano(),
		},
	}, state)
	return nil
}

// checked
func (c *AutoScaleMeta) loadTenants4Test() {
	c.SetupAutoPauseTenant("t1", 1, 4)

	c.setupManualPauseMockTenant("t2", 1, 4, false, 60, nil) // t2
	c.setupManualPauseMockTenant("t3", 1, 4, true, 60, nil)  // t3

	c.setupManualPauseMockTenant("t4", 1, 4, false, 120, NewCpuScaleRule(60, 80, "t4")) // t4 enabled scaleInterval: 120s cpuRule:60%~80%
	c.setupManualPauseMockTenant("t5", 1, 4, false, 60, NewCpuScaleRule(99, 100, "t5")) //0 1
	c.setupManualPauseMockTenant("t6", 1, 4, false, 60, NewCpuScaleRule(99, 100, "t6"))
	c.setupManualPauseMockTenant("t7", 1, 4, false, 60, NewCpuScaleRule(20, 80, "t7"))
	c.setupAutoPauseMockTenant("t8", 1, 4, false, 60, 60, NewCpuScaleRule(40, 80, "t8"), TenantStatePaused)
	c.setupAutoPauseMockTenant("t9", 1, 4, false, 60, 60, NewCpuScaleRule(40, 80, "t9"), TenantStateResumed)
	c.setupAutoPauseMockTenant("t10", 1, 4, false, 120, 120, NewCpuScaleRule(40, 80, "t10"), TenantStatePaused)
	c.setupAutoPauseMockTenant("t11", 1, 4, false, 120, 120, NewCpuScaleRule(40, 80, "t11"), TenantStateResumed)

	/// TODO load tenants from config of control panel
}

type TenantInfoProvider interface {
	GetTenantInfoOfPod(podName string) (string, int64)
	GetTenantScaleIntervalSec(tenant string) (int, bool) // return interval, hasErr
}

// checked
func (c *AutoScaleMeta) GetTenantInfoOfPod(podName string) (string, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.PodDescMap[podName]
	if !ok {
		return "", 0
	}
	return v.GetTenantInfo()
}

// checked
func (c *AutoScaleMeta) GetTenantScaleIntervalSec(tenant string) (int, bool /*hasErr*/) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.tenantMap[tenant]
	if !ok {
		return 0, true
	}
	return v.GetScaleIntervalSec(), false
}

func (c *AutoScaleMeta) GetTenantAutoPauseIntervalSec(tenant string) (int, bool /*hasErr*/) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.tenantMap[tenant]
	if !ok {
		return 0, true
	}
	return v.GetScaleIntervalSec(), false
}

func (c *AutoScaleMeta) CopyPodDescMap() map[string]*PodDesc {
	c.mu.Lock()
	ret := make(map[string]*PodDesc, len(c.PodDescMap))

	for k, v := range c.PodDescMap {
		ret[k] = v
	}
	c.mu.Unlock()
	return ret
}

func (c *AutoScaleMeta) ScanStateOfPods(atStartUp bool) {
	// Logger.Infof("[ScanStateOfPods] begin")
	t1 := time.Now().UnixMilli()
	c.mu.Lock()
	pods := make([]*PodDesc, 0, len(c.PodDescMap))
	for _, v := range c.PodDescMap {
		pods = append(pods, v)
	}
	c.mu.Unlock()
	t2 := time.Now().UnixMilli()
	Logger.Infof("[ScanStateOfPods] ready to scan pods , pods_cnt: %v\n", len(pods))
	var wg sync.WaitGroup
	// statesDeltaMap := make(map[string]string)
	// var muOfStatesDeltaMap sync.Mutex
	for _, v := range pods {
		if v.IP != "" {
			wg.Add(1)
			go func(podDesc *PodDesc) {
				defer wg.Done()
				// resp, err := GetCurrentTenant(podDesc.IP)

				podDesc.ApiGetCurrentTenantAndCorrect(c, atStartUp)
				// Logger.Debugf("[ScanStateOfPods]GetCurrentTenant tenant:%v pod:%v resp:%v", podDesc.TenantName, podDesc.Name, resp.String())
				// if err != nil {
				// 	Logger.Errorf("[ScanStateOfPods]failed to GetCurrentTenant, podname: %v ip: %v, error: %v", podDesc.Name, podDesc.IP, err.Error())
				// } else {
				// 	// if resp.GetTenantID() != "" {
				// 	if !resp.IsUnassigning {
				// 		c.mu.Lock()
				// 		c.updateLocalMetaPodOfTenant(podDesc.Name, podDesc, resp.GetTenantID(), resp.StartTime)
				// 		c.mu.Unlock()
				// 		// muOfStatesDeltaMap.Lock()
				// 		// if resp.GetTenantID() == "" {
				// 		// 	statesDeltaMap[podDesc.Name] = ConfigMapPodStateStr(CmRnPodStateUnassigned, "")
				// 		// } else {
				// 		// 	statesDeltaMap[podDesc.Name] = ConfigMapPodStateStr(CmRnPodStateAssigned, resp.GetTenantID())
				// 		// }
				// 		// muOfStatesDeltaMap.Unlock()
				// 	}
				// 	// }
				// }

			}(v)
		}
	}
	wg.Wait()
	if atStartUp {
		tenants := c.GetTenants()
		for _, tenant := range tenants {
			tenant.SortPodsAtStartUp()
		}
	}
	Logger.Infof("[ScanStateOfPods] finish scan of pods: %v scan_cost: %vms lock_cost: %vms at_startup:%v\n", len(pods), time.Now().UnixMilli()-t2, t2-t1, atStartUp)
	// c.setConfigMapStateBatch(statesDeltaMap)
}

func (c *AutoScaleMeta) ScanStateOfPodsAtRuntime() {
	//TODO implements since PodWatchLoop and remove/add_Pods_from/into_tenants can correct info, may be this func is unnecessary
}

func (c *AutoScaleMeta) GetTenants() []*TenantDesc {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret := make([]*TenantDesc, 0, len(c.tenantMap))
	for _, v := range c.tenantMap {
		ret = append(ret, v)
	}
	return ret
}

func (c *AutoScaleMeta) CopyTenantsMap() map[string]*TenantDesc {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret := make(map[string]*TenantDesc)
	for k, v := range c.tenantMap {
		ret[k] = v
	}
	return ret
}

func (c *AutoScaleMeta) GetTenantDesc(tenant string) *TenantDesc {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret, ok := c.tenantMap[tenant]
	if !ok {
		return nil
	}
	return ret
}

func (c *AutoScaleMeta) GetTenantNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret := make([]string, 0, len(c.tenantMap))
	for _, v := range c.tenantMap {
		ret = append(ret, v.Name)
	}
	return ret
}

func (c *AutoScaleMeta) AsyncPause(tenant string, tsContainer *TimeSeriesContainer) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.tenantMap[tenant]
	if !ok {
		return false
	}
	if v.SyncStatePausing() {
		Logger.Infof("[AutoScaleMeta] Pausing %v", tenant)
		go c.removePodFromTenant(v.GetCntOfPods(), tenant, tsContainer, true)
		return true
	} else {
		return false
	}
}

func (c *AutoScaleMeta) AsyncResume(tenant string, tsContainer *TimeSeriesContainer, resultChan chan<- int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.tenantMap[tenant]
	if !ok {
		return false
	}
	if v.SyncStateResuming() {
		Logger.Infof("[AutoScaleMeta] Resuming %v", tenant)
		// TODO ensure there is no pods now
		go c.addPodIntoTenant(v.GetInitCntOfPod(), tenant, tsContainer, true, resultChan)
		return true
	} else {
		return false
	}
}

// checked
func (c *AutoScaleMeta) GetTopology(tenant string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.tenantMap[tenant]
	if !ok {
		return nil
	}
	return v.GetPodAddrs()
}

func (c *AutoScaleMeta) GetTenantState(tenant string) (bool, int32, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.tenantMap[tenant]
	if !ok {
		return false, TenantStateUnknown, 0
	}
	return true, v.GetState(), v.GetCntOfPods()
}

func (cur *AutoScaleMeta) CreateOrGetPodDesc(podName string, createOrGet bool) *PodDesc {
	val, ok := cur.PodDescMap[podName]
	if !ok {
		// if !createIfNotExist {
		// 	return nil
		// }
		if !createOrGet { // should create but get is true
			return nil
		}
		ret := &PodDesc{}
		cur.PodDescMap[podName] = ret
		return ret
	} else {
		if createOrGet { // should get but create is true
			return nil
		}
		return val
	}
}

// Used by controller
func (c *AutoScaleMeta) addPreWarmFromPending(podName string, desc *PodDesc) {
	Logger.Infof("[AutoScaleMeta]addPreWarmFromPending %v", podName)
	// c.pendingCnt-- // dec pending cnt
	c.PrewarmPool.putWarmedPod("", desc, true)
	// desc.switchState(PodStateInit, PodStateUnassigned)
	// c.setConfigMapState(podName, CmRnPodStateUnassigned, "")
}

func (c *AutoScaleMeta) handleChangeOfPodIP(pod *v1.Pod) {
	// TODO implements
}

// update pods when loadpods when boot and delta events during runtime
// Used by controller
func (c *AutoScaleMeta) UpdatePod(pod *v1.Pod) {
	name := pod.Name
	c.mu.Lock()
	defer c.mu.Unlock()
	podDesc, ok := c.PodDescMap[name]
	Logger.Infof("[updatePod] %v cur_ip:%v", name, pod.Status.PodIP)
	if !ok { // new pod
		// c.pendingCnt++ // inc pending cnt
		// c.PrewarmPool.addCntOfPending(1)
		// state := PodStateInit
		// tenantName := ""
		// cmRnPodState := CmRnPodStateUnknown
		// if pod.Status.PodIP != "" {
		// 	// cmRnPodState, tenantName = c.GetRnPodStateAndTenantFromCM(name) // if the state in config is out of date, it will be corrected in scanStateOfPods() later.
		// 	// /// TODO handle ssigning case ,  merge podstate/CmRnPodState together
		// 	// if cmRnPodState != CmRnPodStateUnknown && cmRnPodState == CmRnPodStateAssigned {
		// 	// 	state = PodStateAssigned
		// 	// }
		// }
		podDesc = &PodDesc{Name: name, IP: pod.Status.PodIP}
		c.PodDescMap[name] = podDesc

		if pod.Status.PodIP != "" {
			c.addPreWarmFromPending(name, podDesc)
			Logger.Infof("[UpdatePod]addPreWarmFromPending %v: %v state: unknown", name, pod.Status.PodIP)
		}

		// // pod is ready if state is PodStateUnassigned/PodStateAssigned
		// if state == PodStateUnassigned {
		// 	c.addPreWarmFromPending(name, podDesc)
		// 	Logger.Infof("[UpdatePod]addPreWarmFromPending %v: %v", name, pod.Status.PodIP)
		// } else if state == PodStateAssigned {
		// 	c.pendingCnt--
		// 	c.updateLocalMetaPodOfTenant(name, podDesc, tenantName, 0)
		// 	Logger.Infof("[UpdatePod]updateLocalMetaPodOfTenant %v: %v tenant: %v", name, pod.Status.PodIP, tenantName)
		// }

	} else {
		if podDesc.Name == "" {
			//TODO handle
			Logger.Errorf("[UpdatePod]exception case of Pod %v", name)
		} else {
			if podDesc.IP == "" {
				if pod.Status.PodIP != "" {
					podDesc.IP = pod.Status.PodIP
					c.addPreWarmFromPending(name, podDesc)
					Logger.Infof("[UpdatePod]preWarm Pod %v: %v", name, pod.Status.PodIP)
				} else {
					Logger.Infof("[UpdatePod]preparing Pod %v", name)
				}

			} else {
				if pod.Status.PodIP == "" {
					Logger.Errorf("[UpdatePod]strange case: pod used to has ip, but now it doesn't %v", name)
				} else {
					if podDesc.IP != pod.Status.PodIP {
						podDesc.IP = pod.Status.PodIP
						c.handleChangeOfPodIP(pod)
						Logger.Warnf("[warn][UpdatePod]ipChange Pod %v: %v -> %v", name, podDesc.IP, pod.Status.PodIP)
					} else {
						Logger.Infof("[UpdatePod]keep Pod %v", name)
					}
				}
			}
		}
	}
}

/// WHEN TO PREPARE NEW POD?
/// 1.tenant want scale out but there is no prewarmed pods.
/// 2.periodical task detect cnt of prewarmed is too low. max_active_assign_cnt+current =(expect) bufcnt

// TODO refine lock logic to prevent race
// TODO make it non-blocking between tenants
// there should not be more than one threads calling this for a same tenant
func (c *AutoScaleMeta) ResizePodsOfTenant(from int, target int, tenant string, tsContainer *TimeSeriesContainer) {
	Logger.Infof("[AutoScaleMeta]ResizePodsOfTenant from %v to %v , tenant:%v", from, target, tenant)
	// TODO assert and validate "from" equal to current cntOfPod
	if target > from {
		c.addPodIntoTenant(target-from, tenant, tsContainer, false, nil)
	} else if target < from {
		c.removePodFromTenant(from-target, tenant, tsContainer, false)
	}
}

func (c *AutoScaleMeta) removePodFromCluster(podDesc *PodDesc) {
	podName := podDesc.Name
	oldTenant, _ := podDesc.GetTenantInfo()
	Logger.Infof("[AutoScaleMeta]removePodFromCluster pod:%v tenant:%v", podName, oldTenant)
	// remove old pod of tenant info
	var ok bool
	var oldTenantDesc *TenantDesc
	if oldTenant == "" {
		oldTenantDesc = c.PrewarmPool.WarmedPods
	} else {
		oldTenantDesc, ok = c.tenantMap[oldTenant]
		if !ok {
			oldTenantDesc = nil
		}
	}
	if oldTenantDesc != nil {
		oldTenantDesc.RemovePod(podName)
	}
	// remove podinfo from cluster
	podDesc.ClearTenantInfo()
	// c.mu.Lock()
	delete(c.PodDescMap, podDesc.Name)
	// c.mu.Unlock()
}

func (c *AutoScaleMeta) updateLocalMetaPodOfTenant(podName string, podDesc *PodDesc, tenant string, startTimeOfAssign int64) {
	Logger.Infof("[AutoScaleMeta]updateLocalMetaPodOfTenant pod:%v tenant:%v", podName, tenant)
	// remove old pod of tenant info
	oldTenant, _ := podDesc.GetTenantInfo()
	var ok bool
	if oldTenant != tenant {
		var oldTenantDesc *TenantDesc
		if oldTenant == "" {
			oldTenantDesc = c.PrewarmPool.WarmedPods
		} else {
			oldTenantDesc, ok = c.tenantMap[oldTenant]
			if !ok {
				oldTenantDesc = nil
			}
		}
		if oldTenantDesc != nil {
			oldTenantDesc.RemovePod(podName)
		}
	}

	// c.mu.Lock()
	// defer c.mu.Unlock()
	var newTenantDesc *TenantDesc
	if tenant != "" {
		newTenantDesc, ok = c.tenantMap[tenant]

		if !ok {
			if OptionRunMode == RunModeLocal || OptionRunMode == RunModeServeless {
				Logger.Infof("[AutoScaleMeta][updateLocalMetaPodOfTenant]no such tenant:%v, do auto register", tenant)
				c.setupAutoPauseTenantWithStateExtraArgs(tenant, DefaultMinCntOfPod, DefaultMaxCntOfPod, TenantStateResumed, false)
				newTenantDesc, ok = c.tenantMap[tenant]
			} else {
				///TODO consider dedicated case
			}
		} else {
			if OptionRunMode == RunModeLocal || OptionRunMode == RunModeServeless {
				if !c.IsRuntimeReady.Load() {
					newTenantDesc.SetState(TenantStateResumed)
				} else {
					if newTenantDesc.GetState() != TenantStateResumed {
						Logger.Errorf("[AutoScaleMeta][updateLocalMetaPodOfTenant]unexpected tenant state:%v tenant:%v", TenantState2String(newTenantDesc.GetState()), tenant)
						newTenantDesc.SetState(TenantStateResumed)
					}
				}
			} else {
				///TODO consider dedicated case deeper
				if newTenantDesc.GetState() != TenantStateResumed {
					Logger.Errorf("[AutoScaleMeta][updateLocalMetaPodOfTenant]unexpected tenant state:%v tenant:%v", TenantState2String(newTenantDesc.GetState()), tenant)
					newTenantDesc.SetState(TenantStateResumed)
				}
			}
		}
	} else {
		newTenantDesc = c.PrewarmPool.WarmedPods
		ok = true
	}
	if ok {
		if startTimeOfAssign != 0 {
			newTenantDesc.SetPodWithTenantInfo(podName, podDesc, startTimeOfAssign)
		} else {
			newTenantDesc.SetPod(podName, podDesc)
		}
	} else {
		Logger.Errorf("wild pod found! pod:%v , tenant: %v", podName, tenant)
	}
}

// return cnt fail to add
// -1 is error
func (c *AutoScaleMeta) addPodIntoTenant(addCnt int, tenant string, tsContainer *TimeSeriesContainer, isResume bool, resultChan chan<- int) int {
	Logger.Infof("[AutoScaleMeta][addPodIntoTenant] %v %v ", addCnt, tenant)

	c.mu.Lock()
	// check if tenant is valid again to prevent it has been removed
	tenantDesc, ok := c.tenantMap[tenant]
	c.mu.Unlock()
	if !ok {
		if resultChan != nil {
			resultChan <- 1
		}
		return -1
	}
	tenantDesc.ResizeMu.Lock()
	defer tenantDesc.ResizeMu.Unlock()
	c.mu.Lock()
	// check validation of state
	if isResume { // remove all pods of tenant if we want pause
		state := tenantDesc.GetState()
		if state != TenantStateResuming {
			// ERROR!!!

			Logger.Errorf("[error][AutoScaleMeta][addPodIntoTenant] failed to resume: 'tenantDesc.GetState() != TenantStateResuming', state:%v \n ", state)
			c.mu.Unlock()
			if resultChan != nil {
				resultChan <- 1
			}
			return -1
		}
	} else {
		state := tenantDesc.GetState()
		if state != TenantStateResumed {
			// ERROR!!!
			Logger.Errorf("[error][AutoScaleMeta][addPodIntoTenant] failed: 'tenantDesc.GetState() != TenantStateResumed', state:%v \n ", state)
			c.mu.Unlock()
			if resultChan != nil {
				resultChan <- 1
			}
			return -1
		}
	}

	podsToAssign, failCnt := c.PrewarmPool.getWarmedPods(tenant, addCnt)
	exceptionCnt := 0

	for _, pod2assign := range podsToAssign {
		Logger.Infof("[AutoScaleMeta][addPodIntoTenant] podsToAssign(name, ip): %v %v", pod2assign.Name, pod2assign.IP)
	}

	c.mu.Unlock()

	// statesDeltaMap := make(map[string]string)
	undoList := make([]*PodDesc, 0, addCnt)
	var localMu sync.Mutex
	var apiWg sync.WaitGroup
	apiWg.Add(len(podsToAssign))
	for _, v := range podsToAssign {
		// TODO async call grpc assign api
		v.isStateChanging.Store(true)
		defer v.isStateChanging.Store(false)
		go func(v *PodDesc) {
			defer apiWg.Done()
			resp, err := v.AssignTenantWithMockConf(tenant)
			localMu.Lock()
			defer localMu.Unlock()
			if err != nil || resp.HasErr {
				// HandleAssignError
				if err != nil { // grpc error, undo
					Logger.Errorf("[error][AutoScaleMeta][addPodIntoTenant] grpc error, undo , err: %v", err.Error())
					undoList = append(undoList, v)
				} else { // app api error , correct its state
					Logger.Errorf("[error][AutoScaleMeta][addPodIntoTenant] app api error , err: %v", resp.ErrInfo)
					if !resp.IsUnassigning {
						c.mu.Lock()
						c.updateLocalMetaPodOfTenant(v.Name, v, resp.TenantID, resp.StartTime)
						c.mu.Unlock()
					} else { /// TODO consider it deeper
						HandleUnassingCase(c, resp.TenantID, v, tsContainer)
					}
				}

			} else {
				c.mu.Lock()

				// statesDeltaMap[v.Name] = ConfigMapPodStateStr(CmRnPodStateAssigned, tenant)

				// clear dirty metrics
				c.tenantMap[tenant].SetPodWithTenantInfo(v.Name, v, resp.StartTime) // TODO Do we need setPod in early for-loop
				tsContainer.ResetMetricsOfPod(v.Name)
				c.mu.Unlock()
			}
		}(v)
	}
	apiWg.Wait()

	// undo failed works
	c.mu.Lock()
	for _, v := range undoList {
		_, ok := c.PodDescMap[v.Name]
		if ok {
			c.PrewarmPool.putWarmedPod(tenant, v, false) // TODO do we need treat failed pods as anomaly group, so that we handle them differently from normal ones.
		} else {
			Logger.Warnf("[AutoScaleMeta][addPodIntoTenant]exception case: pod %v has beed deleted by k8s", v.Name)
			exceptionCnt++
		}
		failCnt++
	}
	if isResume {
		tenantDesc.SyncStateResumed()
	}
	c.mu.Unlock()
	if len(undoList) != 0 || exceptionCnt != 0 {
		Logger.Infof("[AutoScaleMeta][addPodIntoTenant] exceptionCnt:%v len(undoList):%v", exceptionCnt, len(undoList))
	}
	// c.setConfigMapStateBatch(statesDeltaMap)
	if resultChan != nil {
		resultChan <- failCnt
	}
	return failCnt
}

func HandleUnassingCase(c *AutoScaleMeta, curtenant string, v *PodDesc, tsContainer *TimeSeriesContainer) {
	go func(c *AutoScaleMeta, curtenant string, v *PodDesc, tsContainer *TimeSeriesContainer) {
		time.Sleep(60 * time.Second)
		c.mu.Lock()
		c.PrewarmPool.putWarmedPod(curtenant, v, false)
		tsContainer.ResetMetricsOfPod(v.Name)
		c.mu.Unlock()
	}(c, curtenant, v, tsContainer)
}

func (c *AutoScaleMeta) removePodFromTenant(removeCnt int, tenant string, tsContainer *TimeSeriesContainer, isPause bool) int {
	Logger.Infof("[AutoScaleMeta][removePodFromTenant] %v %v ", removeCnt, tenant)

	c.mu.Lock()
	// check if tenant is valid again to prevent it has been removed
	tenantDesc, ok := c.tenantMap[tenant]
	c.mu.Unlock()
	if !ok {
		return -1
	}
	tenantDesc.ResizeMu.Lock()
	defer tenantDesc.ResizeMu.Unlock()
	c.mu.Lock() // tenantDesc.ResizeMu.Lock() always before c.mu.Lock(), to prevent dead lock between  tenantDesc.ResizeMu and c.mu.Lock()
	// check validation of state
	if isPause { // remove all pods of tenant if we want pause
		removeCnt = tenantDesc.GetCntOfPods()
		state := tenantDesc.GetState()
		if state != TenantStatePausing {
			// ERROR!!!
			Logger.Errorf("[error][AutoScaleMeta][removePodFromTenant] failed to pause: 'tenantDesc.GetState() != TenantStatePausing', state:%v \n ", state)
			c.mu.Unlock()
			return -1
		}
	} else {
		state := tenantDesc.GetState()
		if state != TenantStateResumed {
			// ERROR!!!
			Logger.Errorf("[error][AutoScaleMeta][removePodFromTenant] failed: 'tenantDesc.GetState() != TenantStateResumed', state:%v \n ", state)
			c.mu.Unlock()
			return -1
		}
	}

	cnt := removeCnt
	exceptionCnt := 0
	podsToUnassign := make([]*PodDesc, 0, removeCnt)
	cnt, podsToUnassign = tenantDesc.PopPods(cnt, podsToUnassign)

	c.mu.Unlock()
	undoList := make([]*PodDesc, 0, removeCnt)
	// statesDeltaMap := make(map[string]string)
	var localMu sync.Mutex
	var apiWg sync.WaitGroup
	apiWg.Add(len(podsToUnassign))
	for _, v := range podsToUnassign {
		// TODO async call grpc unassign api
		v.isStateChanging.Store(true)
		defer v.isStateChanging.Store(false)
		go func(v *PodDesc) {
			defer apiWg.Done()
			resp, err := v.UnassignTenantWithMockConf(tenant, isPause)
			localMu.Lock()
			defer localMu.Unlock()
			if err != nil || resp.HasErr {
				// HandleUnassignError
				if err != nil { // grpc error, undo
					Logger.Errorf("[error][AutoScaleMeta][removePodFromTenant] grpc error, undo , err: %v", err.Error())
					undoList = append(undoList, v)
				} else { // app api error , correct its state
					Logger.Errorf("[error][AutoScaleMeta][removePodFromTenant] app api error, undo , err: %v", resp.ErrInfo)
					if !resp.IsUnassigning {
						c.mu.Lock()
						c.updateLocalMetaPodOfTenant(v.Name, v, resp.TenantID, resp.StartTime)
						c.mu.Unlock()
					} else { /// TODO consider it deeper
						HandleUnassingCase(c, resp.TenantID, v, tsContainer)
					}
				}
			} else {
				c.mu.Lock()
				// statesDeltaMap[v.Name] = ConfigMapPodStateStr(CmRnPodStateUnassigned, "")
				c.PrewarmPool.putWarmedPod(tenant, v, false)
				tsContainer.ResetMetricsOfPod(v.Name)
				c.mu.Unlock()
			}
		}(v)

	}
	apiWg.Wait()
	// undo failed works
	c.mu.Lock()
	for _, v := range undoList {
		_, ok := c.PodDescMap[v.Name]
		if ok {
			tenantDesc.SetPod(v.Name, v) //no need to set startTimeOfAssign again
		} else {
			Logger.Warnf("[AutoScaleMeta][removePodFromTenant]exception case: pod %v has beed deleted by k8s", v.Name)
			exceptionCnt++
		}
		cnt++
	}
	if isPause {
		tenantDesc.SyncStatePaused()
	}
	c.mu.Unlock()
	if len(undoList) != 0 || exceptionCnt != 0 {
		Logger.Infof("[AutoScaleMeta][removePodFromTenant] exceptionCnt:%v len(undoList):%v", exceptionCnt, len(undoList))
	}
	// c.setConfigMapStateBatch(statesDeltaMap)
	Logger.Debugf("[AutoScaleMeta][removePodFromTenant]done. warmpool.size:%v pods:%v", c.WarmedPods.GetCntOfPods(), c.WarmedPods.GetPodNames())
	return cnt
}

func (c *AutoScaleMeta) HandleK8sDelPodEvent(pod *v1.Pod) bool {
	name := pod.Name
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.PodDescMap[name]
	Logger.Infof("[HandleK8sDelPodEvent] podName:%v, isInMap:%v", name, ok)
	if !ok {
		return true
	} else {
		c.removePodFromCluster(v)
		return false
	}
}

// for test
func (c *AutoScaleMeta) AddPod4Test(podName string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.PodDescMap[podName]
	if ok {
		return false
	}
	// podDesc := c.CreateOrGetPodDesc(podName, true)
	// if podDesc == nil {
	// return false
	// }
	podDesc := &PodDesc{}
	c.PodDescMap[podName] = podDesc
	c.PrewarmPool.putWarmedPod("", podDesc, true)
	// c.PrewarmPods.SetPod(podName, podDesc)
	// c.PrewarmPods.pods[podName] = podDesc

	return true
}

// func (c *AutoScaleMeta) SetupTenantWithDefaultArgs4Test(tenant string) bool {
// 	c.mu.Lock()
// 	defer c.mu.Unlock()
// 	_, ok := c.tenantMap[tenant]
// 	if !ok {
// 		c.tenantMap[tenant] = NewTenantDescDefault(tenant)
// 		return true
// 	} else {
// 		return false
// 	}
// }

func (c *AutoScaleMeta) SetupAutoPauseTenantWithState(tenant string, minPods int, maxPods int, state int32) bool {
	return c.setupAutoPauseTenantWithStateExtraArgs(tenant, minPods, maxPods, state, true)
}

func (c *AutoScaleMeta) setupAutoPauseTenantWithStateExtraArgs(tenant string, minPods int, maxPods int, state int32, needLock bool) bool {
	Logger.Infof("[SetupTenant] SetupTenant(%v, %v, %v)", tenant, minPods, maxPods)
	if needLock {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	_, ok := c.tenantMap[tenant]
	if !ok {
		c.tenantMap[tenant] = NewAutoPauseTenantDescWithState(tenant, minPods, maxPods, state)
		return true
	} else {
		return false
	}
}

func (c *AutoScaleMeta) SetupAutoPauseTenant(tenant string, minPods int, maxPods int) bool {
	return c.SetupAutoPauseTenantWithState(tenant, minPods, maxPods, TenantStatePaused)
}

func (c *AutoScaleMeta) SetupTenantWithConfig(tenant string, confHolder *ConfigOfComputeClusterHolder, state int32) bool {
	Logger.Infof("[SetupTenant] SetupTenantWithConfig(%v, %+v)", tenant, confHolder.Config)
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.tenantMap[tenant]
	if !ok {
		c.tenantMap[tenant] = NewTenantDescWithConfigAndState(tenant, confHolder, state)
		return true
	} else {
		return false
	}
}

// func (c *AutoScaleMeta) UpdateTenant4Test(podName string, newTenant string) bool {
// 	c.mu.Lock()
// 	defer c.mu.Unlock()
// 	podDesc, ok := c.PodDescMap[podName]
// 	if !ok {
// 		return false
// 	} else {
// 		// del old info
// 		tenantDesc, ok := c.tenantMap[podDesc.TenantName]
// 		if !ok {
// 			//TODO maintain idle pods
// 			// return false
// 		} else {

// 			tenantDesc.RemovePod(podName)
// 			// podMap := tenantDesc.pods
// 			// _, ok = podMap[podName]
// 			// if ok {
// 			// 	delete(podMap, podName)
// 			// }

// 		}
// 	}
// 	podDesc.TenantName = newTenant
// 	// var newPodMap map[string]*PodDesc
// 	newTenantDesc, ok := c.tenantMap[newTenant]
// 	if !ok {
// 		return false
// 		// newPodMap = make(map[string]*PodDesc)
// 		// c.TenantMap[newTenant] = newPodMap
// 	}
// 	newTenantDesc.SetPod(podName, podDesc)
// 	return true
// }

// checked
func (c *AutoScaleMeta) ComputeStatisticsOfTenant(tenantName string, tsc *TimeSeriesContainer, caller string, metricsTopic MetricsTopic) ([]AvgSigma, map[string]float64 /* avg_map */, map[string]int64 /* cnt_map*/, map[string]*DescOfPodTimeSeries, *DescOfTenantTimeSeries) {
	c.mu.Lock()

	tenantDesc, ok := c.tenantMap[tenantName]
	if !ok {
		c.mu.Unlock()
		return nil, nil, nil, nil, nil
	} else {
		podsOfTenant := tenantDesc.GetPodNames()
		c.mu.Unlock()
		podCpuMap := make(map[string]float64)
		podPointCntMap := make(map[string]int64)
		descOfTimeSeriesMap := make(map[string]*DescOfPodTimeSeries)
		ret := make([]AvgSigma, CapacityOfStaticsAvgSigma)
		var metricDescOfTenant DescOfTenantTimeSeries
		for _, podName := range podsOfTenant {

			// // FOR DEBUG
			tsc.Dump(podName, metricsTopic)

			statsOfPod, descOfTimeSeries := tsc.GetStatisticsOfPod(podName, metricsTopic)
			if statsOfPod == nil {
				statsOfPod = make([]AvgSigma, CapacityOfStaticsAvgSigma)
				dummyTime := time.Now().Unix() + 86400
				descOfTimeSeries = &DescOfPodTimeSeries{
					MinTime: dummyTime,
					MaxTime: dummyTime,
					Size:    0,
				}
			}

			if metricDescOfTenant.PodCnt == 0 {
				metricDescOfTenant.Init(descOfTimeSeries)
			} else {
				metricDescOfTenant.Agg(descOfTimeSeries)
			}

			// TODO hope for a better idea to handle case of new Pod without metrics
			if len(statsOfPod) > 0 {
				// podCpuMap[podName] = statsOfPod[0].Avg()
				Logger.Infof("[debug]avg %v of pod %v : %v, %v", metricsTopic.String(), podName, statsOfPod[0].Avg(), statsOfPod[0].Cnt())
			}
			if metricsTopic == MetricsTopicCpu {
				for i := range statsOfPod { // make weight even between pods
					statsOfPod[i] = AvgSigma{statsOfPod[i].Avg(), 1}
				}
			}
			if len(statsOfPod) > 0 {
				podCpuMap[podName] = statsOfPod[0].Avg()
				podPointCntMap[podName] = statsOfPod[0].Cnt()
				descOfTimeSeriesMap[podName] = descOfTimeSeries
				// Logger.Infof("[debug]avg cpu of pod %v : %v, %v", podName, statsOfPod[0].Avg(), statsOfPod[0].Cnt())
			}
			Merge(ret, statsOfPod)

		}
		return ret, podCpuMap, podPointCntMap, descOfTimeSeriesMap, &metricDescOfTenant
	}
}

func MockComputeStatisticsOfTenant(coresOfPod int, cntOfPods int, maxCntOfPods int) float64 {
	ts := time.Now().Unix() / 2
	// tsInMins := ts / 60
	return math.Min((math.Sin(float64(ts)/10.0)+1)/2*float64(coresOfPod)*float64(maxCntOfPods)/float64(cntOfPods), 8)
}

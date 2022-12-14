package autoscale

import (
	"log"
	"sync"
)

type ConfigManager struct {
	configMap map[string]*ConfigOfComputeCluster
	mu        sync.Mutex
}

type ConfigOfComputeCluster struct {
	Disabled                 bool                 // triger when modified: pause cluster
	AutoPauseIntervalSeconds int                  // triger when modified: re-range timeseries of metric active_task
	MinCores                 int                  // triger when modified: reload config before next analyze loop
	MaxCores                 int                  // triger when modified: reload config before next analyze loop
	InitCores                int                  // triger when modified: reload config before next analyze loop
	WindowSeconds            int                  // triger when modified: re-range timeseries of metric cpu/mem...
	CpuScaleRules            *CustomScaleRule     // triger when modified: reload config before next analyze loop
	ConfigOfTiDBCluster      *ConfigOfTiDBCluster // triger when modified: instantly reload compute pod's config  TODO handle version change case
}

func (c *ConfigOfComputeCluster) GetInitCntOfPod() int {
	if c.InitCores >= c.MinCores && c.InitCores <= c.MaxCores && c.InitCores%DefaultCoreOfPod == 0 {
		return c.InitCores / DefaultCoreOfPod
	} else {
		log.Printf("[error][ConfigOfComputeCluster] invalid initcores, TiDBCluster: %v ,CoreInfo min:%v max:%v init:%v \n", c.ConfigOfTiDBCluster.Name, c.MinCores, c.MaxCores, c.InitCores)
		return c.MinCores / DefaultCoreOfPod
	}
}

func (c *ConfigOfComputeCluster) GetLowerAndUpperCpuScaleThreshold() (float64, float64) {
	if c.CpuScaleRules != nil {
		return float64(c.CpuScaleRules.Threashold.Min) / 100.0, float64(c.CpuScaleRules.Threashold.Max) / 100.0
	} else {
		log.Printf("[error][ConfigOfComputeCluster]invalid LowerAndUpperCpuScaleThreshold, TiDbCluster: %v CpuRuleInfo min:%v max:%v \n", c.ConfigOfTiDBCluster.Name, c.CpuScaleRules.Threashold.Min, c.CpuScaleRules.Threashold.Max)
		return DefaultLowerLimit, DefaultUpperLimit
	}
}

type ConfigOfComputeClusterHolder struct {
	Config         ConfigOfComputeCluster
	mu             sync.Mutex
	LastModifiedTs int64
}

func (c *ConfigOfComputeClusterHolder) HasChanged(oldTs int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LastModifiedTs > oldTs
}

func (c *ConfigOfComputeClusterHolder) DeepCopy() ConfigOfComputeCluster {
	c.mu.Lock()
	ret := c.Config
	c.mu.Unlock()
	return ret
}

type ConfigOfTiDBCluster struct {
	Name    string // TiDBCluster 的全局唯一 ID
	Version string
	PD      *ConfigOfPD
	TiDB    []*ConfigOfTiDB

	// all fields below may be useless
	Region string
	Tier   string
	Cloud  string
}

type CustomScaleRule struct {
	// only cpu is supported now
	Name string
	// min/max for scaling, unit: % for cpu metric
	Threashold *Threashold
	// window for metric samples
	// 120s~600s (2m~10m)

}

type Threashold struct {
	Min int
	Max int
}

type ConfigOfPD struct {
	Addr string
}

type ConfigOfTiDB struct {
	Name       string
	StatusAddr string

	Zone string
}

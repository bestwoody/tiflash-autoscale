package autoscale

import (
	"fmt"
	"sync"
)

type ConfigManager struct {
	configMap map[string]*ConfigOfComputeCluster
	mu        sync.Mutex
}

// auto: on/off  resume: on/off
/*  HOW TO Initialize state when new Tenant is setup
// AutoPauseIntervalSeconds Disabled InitializedState TargetState      Action
//       0                     false  resumed           resumed        autoscale
//       0                     true   resumed           paused         pause()
//       non-0                 false  paused/custom	    auto pause/resume  ResumeReqFromTidb
//       non-0			       true   resumed	        paused         pause()
*/
type ConfigOfComputeCluster struct {
	Disabled                 bool                 // triger when modified: pause cluster
	AutoPauseIntervalSeconds int                  // triger when modified: re-range timeseries of metric active_task。 zero means non-auto pause
	MinCores                 int                  // triger when modified: reload config before next analyze loop
	MaxCores                 int                  // triger when modified: reload config before next analyze loop
	InitCores                int                  // triger when modified: reload config before next analyze loop
	WindowSeconds            int                  // triger when modified: re-range timeseries of metric cpu/mem...
	CpuScaleRules            *CustomScaleRule     // triger when modified: reload config before next analyze loop
	ConfigOfTiDBCluster      *ConfigOfTiDBCluster // triger when modified: instantly reload compute pod's config  TODO handle version change case
	LastModifiedTs           int64
}

func (c *ConfigOfComputeCluster) Dump() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("ConfigOfComputeCluster{Disabled:%v, AutoPauseIntervalSec:%v, MinCores:%v, MaxCores:%v, InitCores:%v, WindowSec:%v, CpuScaleRules:%v, TidbCluster:%v, LastModifiedTs:%v}",
		c.Disabled, c.AutoPauseIntervalSeconds, c.MinCores, c.MaxCores, c.InitCores, c.WindowSeconds, c.CpuScaleRules.Dump(), c.ConfigOfTiDBCluster.Dump(), c.LastModifiedTs)
}

func (c *ConfigOfComputeCluster) GetInitCntOfPod() int {
	if c.InitCores >= c.MinCores && c.InitCores <= c.MaxCores && c.InitCores%DefaultCoreOfPod == 0 {
		return c.InitCores / DefaultCoreOfPod
	} else {
		Logger.Errorf("[error][ConfigOfComputeCluster] invalid initcores, TiDBCluster: %v ,CoreInfo min:%v max:%v init:%v ", c.ConfigOfTiDBCluster.Name, c.MinCores, c.MaxCores, c.InitCores)
		return c.MinCores / DefaultCoreOfPod
	}
}

func (c *ConfigOfComputeCluster) GetLowerAndUpperCpuScaleThreshold() (float64, float64) {
	if c.CpuScaleRules != nil {
		return float64(c.CpuScaleRules.Threashold.Min) / 100.0, float64(c.CpuScaleRules.Threashold.Max) / 100.0
	} else {
		Logger.Warnf("[warn][ConfigOfComputeCluster]CpuScaleRules is nil, TiDbCluster: %v ", c.ConfigOfTiDBCluster.Name)
		return DefaultLowerLimit, DefaultUpperLimit
	}
}

type ConfigOfComputeClusterHolder struct {
	Config ConfigOfComputeCluster
	mu     sync.Mutex
}

func (c *ConfigOfComputeClusterHolder) HasChanged(oldTs int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Config.LastModifiedTs > oldTs
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
	// TiDB    []*ConfigOfTiDB

	// all fields below may be useless
	// Region string
	// Tier   string
	// Cloud  string
}

func (c *ConfigOfTiDBCluster) Dump() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("ConfigOfTiDBCluster{Name:%v, Version:%v}", c.Name, c.Version)
}

type CustomScaleRule struct {
	// only cpu is supported now
	Name string // only for display
	// min/max for scaling, unit: % for cpu metric
	Threashold *Threashold
	// window for metric samples
	// 120s~600s (2m~10m)

}

func NewCpuScaleRule(minPercent int, maxPerent int, title string) *CustomScaleRule {
	if maxPerent <= minPercent {
		Logger.Errorf("invalid params: maxPerent <= minPercent, title:%v minPercent:%v maxPercent:%v", title, minPercent, maxPerent)
		return nil
	}
	return &CustomScaleRule{
		Name: "cpu",
		Threashold: &Threashold{
			Min: minPercent,
			Max: maxPerent,
		},
	}
}

func (c *CustomScaleRule) Dump() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("CustomScaleRule{Name:%v Threashold:%v}", c.Name, c.Threashold.Dump())
}

type Threashold struct {
	Min int
	Max int
}

func (c *Threashold) Dump() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("Threashold{Min:%v, Max:%v}", c.Min, c.Max)
}

type ConfigOfPD struct {
	Addr string
}

type ConfigOfTiDB struct {
	Name       string
	StatusAddr string

	Zone string
}

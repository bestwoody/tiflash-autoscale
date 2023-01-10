package autoscale

import (
	"math"
)

// func ComputeBestPodsInRuleOfPM(tenantDesc *TenantDesc, cpuusage float64, coreOfPod int) (int, int /*delta*/) {
// 	if tenantDesc == nil {
// 		return -1, 0
// 	}
// 	lowLimit := float64(coreOfPod) * DefaultLowerLimit
// 	upLimit := float64(coreOfPod) * DefaultUpperLimit
// 	if cpuusage >= lowLimit && cpuusage <= upLimit {
// 		return -1, 0
// 	} else {
// 		// mu.Lock()
// 		oldCntOfPods := tenantDesc.GetCntOfPods()
// 		minCntOfPods := tenantDesc.MinCntOfPod
// 		maxCntOfPods := tenantDesc.MaxCntOfPod
// 		// mu.Unlock()
// 		if cpuusage > upLimit {
// 			ret := MinInt(oldCntOfPods+1, maxCntOfPods)
// 			return ret, ret - oldCntOfPods
// 		} else if cpuusage < lowLimit {
// 			ret := MaxInt(oldCntOfPods-1, minCntOfPods)
// 			return ret, ret - oldCntOfPods
// 		} else {
// 			return -1, 0
// 		}
// 	}
// }

func ComputeBestPodsInRuleOfCompute(tenantDesc *TenantDesc, cpuUsageCoresPerPod float64, cpuLowerlimit float64, cpuUpperLimit float64) (int, int /*delta*/) {
	if tenantDesc == nil {
		Logger.Infof("[ComputeBestPodsInRuleOfCompute]tenantDesc == nil")
		return -1, 0
	}
	if cpuLowerlimit >= cpuUpperLimit {
		Logger.Errorf("[ComputeBestPodsInRuleOfCompute]cpuLowerlimit >= cpuUpperLimit")
		return -1, 0
	}
	tenantname := tenantDesc.Name
	coreOfPod := DefaultCoreOfPod
	lowerLimitOfGlobalPercentage := cpuLowerlimit
	upperlimitOfGlobalPercentage := cpuUpperLimit
	lowLimitOfCpuUsage := float64(coreOfPod) * lowerLimitOfGlobalPercentage
	upLimitOfCpuUsage := float64(coreOfPod) * upperlimitOfGlobalPercentage
	if cpuUsageCoresPerPod >= lowLimitOfCpuUsage && cpuUsageCoresPerPod <= upLimitOfCpuUsage {
		return -1, 0
	} else {
		// mu.Lock()
		oldCntOfPods := tenantDesc.GetCntOfPods()
		minCntOfPods := tenantDesc.GetMinCntOfPod()
		maxCntOfPods := tenantDesc.GetMaxCntOfPod()
		if lowLimitOfCpuUsage+upLimitOfCpuUsage == 0 {
			Logger.Infof("[ComputeBestPodsInRuleOfCompute][%v]case#1: lowLimitOfCpuUsage+upLimitOfCpuUsage == 0", tenantname)
			return -1, 0
		}
		logicalTargetCpuUsageInGlobalPercentage := (lowerLimitOfGlobalPercentage + upperlimitOfGlobalPercentage) / 2
		// cpuusage * oldCntOfPods
		logicalTargetCpuCores := cpuUsageCoresPerPod * float64(oldCntOfPods) / logicalTargetCpuUsageInGlobalPercentage
		logicalTargetCntOfPod := logicalTargetCpuCores / float64(coreOfPod)
		var targetCntOfPod int
		if logicalTargetCntOfPod > float64(oldCntOfPods) && logicalTargetCntOfPod < float64(oldCntOfPods)+1 {
			targetCntOfPod = int(logicalTargetCntOfPod + 0.99)
		} else if logicalTargetCntOfPod < float64(oldCntOfPods) && logicalTargetCntOfPod > float64(oldCntOfPods)-1 {
			targetCntOfPod = int(logicalTargetCntOfPod)
		} else {
			targetCntOfPod = int(math.Round(logicalTargetCpuCores / float64(coreOfPod)))
		}
		targetCntOfPod = MaxInt(targetCntOfPod, 1)
		targetCpuUsageCoresPerPod := cpuUsageCoresPerPod * float64(oldCntOfPods) / float64(targetCntOfPod)
		// mu.Unlock()
		// 2 -> 2.1
		// 2 -> 2.9

		// 1, 0.9 -> 2, 0.45
		// 1, UP -> 2, UP/2
		// LOW << UP/2
		// 0.6~0.9 , 0.75

		if cpuUsageCoresPerPod > upLimitOfCpuUsage {

			if targetCntOfPod < oldCntOfPods {
				Logger.Infof("[ComputeBestPodsInRuleOfCompute][%v]case#2: targetCntOfPod < oldCntOfPods, %v vs %v", tenantname, targetCntOfPod, oldCntOfPods)
				return -1, 0
			}
			if targetCpuUsageCoresPerPod <= lowLimitOfCpuUsage {
				// check if targetCpuUsage can triger scale in which will cause fluctuate
				// skip if true
				Logger.Warnf("[ComputeBestPodsInRuleOfCompute][%v]case#3: targetCpuUsage <= lowLimitOfCpuUsage, %v vs %v", tenantname, targetCpuUsageCoresPerPod, lowLimitOfCpuUsage)
				// return -1, 0
				ret := MinInt(oldCntOfPods+1, maxCntOfPods)
				return ret, ret - oldCntOfPods
			}
			ret := MinInt(targetCntOfPod, maxCntOfPods)
			return ret, ret - oldCntOfPods
		} else if cpuUsageCoresPerPod < lowLimitOfCpuUsage {

			if targetCntOfPod > oldCntOfPods {
				Logger.Infof("[ComputeBestPodsInRuleOfCompute][%v]case#4: targetCntOfPod > oldCntOfPods, %v vs %v", tenantname, targetCntOfPod, oldCntOfPods)
				return -1, 0
			}
			if targetCpuUsageCoresPerPod >= upLimitOfCpuUsage {
				// check if targetCpuUsage can triger scale out which will cause fluctuate
				// skip if true
				Logger.Warnf("[ComputeBestPodsInRuleOfCompute][%v]case#5: targetCpuUsage >= upLimitOfCpuUsage, %v vs %v", tenantname, targetCpuUsageCoresPerPod, upLimitOfCpuUsage)
				ret := MaxInt(oldCntOfPods-1, minCntOfPods)
				return ret, ret - oldCntOfPods
				// return -1, 0
			}
			ret := MaxInt(targetCntOfPod, minCntOfPods)
			return ret, ret - oldCntOfPods
		} else {
			return -1, 0
		}
	}
}

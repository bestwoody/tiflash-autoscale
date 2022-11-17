package autoscale

import (
	"log"
	"math"
)

func ComputeBestPodsInRuleOfPM(tenantDesc *TenantDesc, cpuusage float64, coreOfPod int) (int, int /*delta*/) {
	if tenantDesc == nil {
		return -1, 0
	}
	lowLimit := float64(coreOfPod) * DefaultLowerLimit
	upLimit := float64(coreOfPod) * DefaultUpperLimit
	if cpuusage >= lowLimit && cpuusage <= upLimit {
		return -1, 0
	} else {
		// mu.Lock()
		oldCntOfPods := tenantDesc.GetCntOfPods()
		minCntOfPods := tenantDesc.MinCntOfPod
		maxCntOfPods := tenantDesc.MaxCntOfPod
		// mu.Unlock()
		if cpuusage > upLimit {
			ret := MinInt(oldCntOfPods+1, maxCntOfPods)
			return ret, ret - oldCntOfPods
		} else if cpuusage < lowLimit {
			ret := MaxInt(oldCntOfPods-1, minCntOfPods)
			return ret, ret - oldCntOfPods
		} else {
			return -1, 0
		}
	}
}

func ComputeBestPodsInRuleOfCompute(tenantDesc *TenantDesc, cpuusage float64, coreOfPod int) (int, int /*delta*/) {
	if tenantDesc == nil {
		log.Println("[ComputeBestPodsInRuleOfCompute]tenantDesc == nil")
		return -1, 0
	}
	lowerLimitOfGlobalPercentage := DefaultLowerLimit
	upperlimitOfGlobalPercentage := DefaultUpperLimit
	lowLimitOfCpuUsage := float64(coreOfPod) * lowerLimitOfGlobalPercentage
	upLimitOfCpuUsage := float64(coreOfPod) * upperlimitOfGlobalPercentage
	if cpuusage >= lowLimitOfCpuUsage && cpuusage <= upLimitOfCpuUsage {
		return -1, 0
	} else {
		// mu.Lock()
		oldCntOfPods := tenantDesc.GetCntOfPods()
		minCntOfPods := tenantDesc.MinCntOfPod
		maxCntOfPods := tenantDesc.MaxCntOfPod
		if lowLimitOfCpuUsage+upLimitOfCpuUsage == 0 {
			log.Println("[ComputeBestPodsInRuleOfCompute]lowLimitOfCpuUsage+upLimitOfCpuUsage == 0")
			return -1, 0
		}
		logicalTargetCpuUsageInGlobalPercentage := (lowerLimitOfGlobalPercentage + upperlimitOfGlobalPercentage) / 2
		// cpuusage * oldCntOfPods
		logicalTargetCpuCores := cpuusage * float64(oldCntOfPods) / logicalTargetCpuUsageInGlobalPercentage
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
		targetCpuUsage := cpuusage * float64(oldCntOfPods) / float64(targetCntOfPod)
		// mu.Unlock()
		// 2 -> 2.1
		// 2 -> 2.9

		// 1, 0.9 -> 2, 0.45
		// 1, UP -> 2, UP/2
		// LOW << UP/2
		// 0.6~0.9 , 0.75

		if cpuusage > upLimitOfCpuUsage {
			if targetCpuUsage <= lowLimitOfCpuUsage {
				// check if targetCpuUsage can triger scale in which will cause fluctuate
				// skip if true
				log.Printf("[ComputeBestPodsInRuleOfCompute]targetCpuUsage <= lowLimitOfCpuUsage, %v vs %v\n", targetCpuUsage, lowLimitOfCpuUsage)
				return -1, 0
			}
			if targetCntOfPod < oldCntOfPods {
				log.Printf("[ComputeBestPodsInRuleOfCompute]targetCntOfPod < oldCntOfPods, %v vs %v\n", targetCntOfPod, oldCntOfPods)
				return -1, 0
			}
			ret := MinInt(targetCntOfPod, maxCntOfPods)
			return ret, ret - oldCntOfPods
		} else if cpuusage < lowLimitOfCpuUsage {
			if targetCpuUsage >= upLimitOfCpuUsage {
				// check if targetCpuUsage can triger scale out which will cause fluctuate
				// skip if true
				log.Printf("[ComputeBestPodsInRuleOfCompute]targetCpuUsage >= upLimitOfCpuUsage, %v vs %v\n", targetCpuUsage, upLimitOfCpuUsage)
				return -1, 0
			}
			if targetCntOfPod > oldCntOfPods {
				log.Printf("[ComputeBestPodsInRuleOfCompute]targetCntOfPod > oldCntOfPods, %v vs %v\n", targetCntOfPod, oldCntOfPods)
				return -1, 0
			}
			ret := MaxInt(targetCntOfPod, minCntOfPods)
			return ret, ret - oldCntOfPods
		} else {
			return -1, 0
		}
	}
}

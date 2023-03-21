package autoscale

import (
	"fmt"
	"strconv"
	"testing"
)

func isInRangeInt(v int, min int, max int) bool {
	return v >= min && v <= max
}

func InitTestEnv() {
	LogMode = LogModeLocalTest
	InitZapLogger()
}

func TestComputeBestCore3(t *testing.T) {
	InitTestEnv()
	minPods := 1
	maxPods := 4
	tenantDesc := NewAutoPauseTenantDescWithState("test1", minPods, maxPods, TenantStateResumed, "")
	tenantDesc.SetPod("p1", &PodDesc{Name: "p1"})
	tenantDesc.SetPod("p2", &PodDesc{Name: "p3"})
	tenantDesc.SetPod("p3", &PodDesc{Name: "p3"})
	target, delta := ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.1376888115180687), 0.2, 0.5)
	assertEqual(t, target, 1)
	assertEqual(t, delta, -2)

}

func TestComputeBestCore(t *testing.T) {
	InitTestEnv()
	minPods := 1
	maxPods := 4
	tenantDesc := NewAutoPauseTenantDescWithState("test1", minPods, maxPods, TenantStateResumed, "")
	tenantDesc.SetPod("p1", &PodDesc{Name: "p1"})
	target, delta := ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(1.0), 0.6, 0.8)
	assertEqual(t, target, 2)
	assertEqual(t, delta, 1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.9), 0.6, 0.8)
	assertEqual(t, target, 2)
	assertEqual(t, delta, 1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.4), 0.6, 0.8)
	assertEqual(t, target == 1 || target == -1, true)
	assertEqual(t, delta, 0)
	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.1), 0.6, 0.8)
	assertEqual(t, target == 1 || target == -1, true)
	assertEqual(t, delta, 0)
	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0), 0.6, 0.8)
	assertEqual(t, target == 1 || target == -1, true)
	assertEqual(t, delta, 0)

	// resize CntOfPods from 1 to 2
	tenantDesc.SetPod("p2", &PodDesc{Name: "p2"})
	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(1.0), 0.6, 0.8)
	assertEqual(t, target, 3)
	assertEqual(t, delta, 1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.9), 0.6, 0.8)
	assertEqual(t, target, 3)
	assertEqual(t, delta, 1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0), 0.6, 0.8)
	assertEqual(t, target, 1)
	assertEqual(t, delta, -1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.1), 0.6, 0.8)
	Logger.Infof("%v %v", target, delta)
	assertEqual(t, target, 1)
	assertEqual(t, delta, -1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.5), 0.6, 0.8)
	Logger.Infof("%v %v", target, delta)
	assertEqual(t, target, 1)
	assertEqual(t, delta, -1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.6), 0.6, 0.8)
	Logger.Infof("%v %v", target, delta)
	assertEqual(t, target == 3 || target == -1, true)
	assertEqual(t, delta, 0)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.7), 0.6, 0.8)
	Logger.Infof("%v %v", target, delta)
	assertEqual(t, target == 3 || target == -1, true)
	assertEqual(t, delta, 0)

	// resize CntOfPods from 2 to 3
	tenantDesc.SetPod("p3", &PodDesc{Name: "p3"})
	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(1.0), 0.6, 0.8)
	assertEqual(t, target, 4)
	assertEqual(t, delta, 1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.9), 0.6, 0.8)
	assertEqual(t, target, 4)
	assertEqual(t, delta, 1)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.7), 0.6, 0.8)
	assertEqual(t, target == 3 || target == -1, true)
	assertEqual(t, delta, 0)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.1), 0.6, 0.8)
	fmt.Printf("%v %v", target, delta)
	assertEqual(t, target < 3 && isInRangeInt(target, minPods, maxPods), true)
	assertEqual(t, delta <= -1, true)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.3), 0.6, 0.8)
	fmt.Printf("%v %v", target, delta)
	assertEqual(t, target < 3 && isInRangeInt(target, minPods, maxPods), true)
	assertEqual(t, delta <= -1, true)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.5), 0.6, 0.8)
	fmt.Printf("%v %v", target, delta)
	assertEqual(t, target < 3 && isInRangeInt(target, minPods, maxPods), true)
	assertEqual(t, delta <= -1, true)

	// resize CntOfPods from 3 to 4
	tenantDesc.SetPod("p4", &PodDesc{Name: "p4"})
	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(1.0), 0.6, 0.8)
	assertEqual(t, target == 4 || target == -1, true)
	assertEqual(t, delta, 0)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.9), 0.6, 0.8)
	assertEqual(t, target == 4 || target == -1, true)
	assertEqual(t, delta, 0)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.7), 0.6, 0.8)
	assertEqual(t, target == 4 || target == -1, true)
	assertEqual(t, delta, 0)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.1), 0.6, 0.8)
	fmt.Printf("%v %v", target, delta)
	assertEqual(t, target < 4 && isInRangeInt(target, minPods, maxPods), true)
	assertEqual(t, delta <= -1, true)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.3), 0.6, 0.8)
	fmt.Printf("%v %v", target, delta)
	assertEqual(t, target < 4 && isInRangeInt(target, minPods, maxPods), true)
	assertEqual(t, delta <= -1, true)

	target, delta = ComputeBestPodsInRuleOfCompute(
		tenantDesc, ComputeCpuUsageCoresPerPod(0.5), 0.6, 0.8)
	fmt.Printf("%v %v", target, delta)
	assertEqual(t, target < 4 && isInRangeInt(target, minPods, maxPods), true)
	assertEqual(t, delta <= -1, true)
}

func TestComputeBestCore2(t *testing.T) {
	InitTestEnv()
	// testComputeBestCoreCommon(t, 0.8, 0.2)
	testComputeBestCoreCommon(t, 0.2, 0.8)
	testComputeBestCoreCommon(t, 0.6, 0.8)
	testComputeBestCoreCommon(t, 0.0, 0.01)
	testComputeBestCoreCommon(t, 0.99, 1.0)
}

func testComputeBestCoreCommon(t *testing.T, minRatio float64, maxRatio float64) {
	minPods := 1
	maxPods := 4
	// minRatio := float64(0.2)
	// maxRatio := float64(0.8)
	tenantDesc := NewAutoPauseTenantDescWithState("test1", minPods, maxPods, TenantStateResumed, "")
	testUsagesArr := []float64{0.0, 0.001, 0.01, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.99, 0.999, 1.0}
	cnt1 := 0
	cnt2 := 0
	cnt3 := 0
	cnt4 := 0
	cnt5 := 0
	for i := 1; i <= maxPods; i++ {
		tenantDesc.SetPod("p1"+strconv.Itoa(i), &PodDesc{Name: "p1" + strconv.Itoa(i)})
		for _, cpuRatio := range testUsagesArr {
			target, delta := ComputeBestPodsInRuleOfCompute(
				tenantDesc, ComputeCpuUsageCoresPerPod(cpuRatio), minRatio, maxRatio)
			if cpuRatio >= minRatio && cpuRatio <= maxRatio {
				cnt1++
				assertEqual(t, target == i || target == -1, true)
				assertEqual(t, delta, 0)
			} else if cpuRatio < minRatio {
				if i != minPods {
					cnt2++
					assertEqual(t, target < i && isInRangeInt(target, minPods, maxPods), true)
					assertEqual(t, delta <= -1, true)
				} else {
					cnt3++
					assertEqual(t, target == i || target == -1, true)
					assertEqual(t, delta, 0)
				}
			} else if cpuRatio > maxRatio {
				if i != maxPods {
					cnt4++
					assertEqual(t, target > i && isInRangeInt(target, minPods, maxPods), true)
					assertEqual(t, delta >= 1, true)
				} else {
					cnt5++
					assertEqual(t, target == i || target == -1, true)
					assertEqual(t, delta, 0)
				}
			} else {
				panic("impossible branch")
			}
		}
	}
	t.Logf("[TestComputeBestCore2]stats: %v %v %v %v %v\n", cnt1, cnt2, cnt3, cnt4, cnt5)

}

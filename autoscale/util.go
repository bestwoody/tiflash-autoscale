package autoscale

import (
	"fmt"
	"testing"
)

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	// var message string
	if a == b {
		return
	}
	// if len(message) == 0 {
	message := fmt.Sprintf("%v != %v", a, b)
	// }
	panic(message)
	// t.Fatal(message)
}

func ComputeCpuUsageCoresPerPod(cpuUsageRatio float64) float64 {
	return cpuUsageRatio * float64(DefaultCoreOfPod)
}

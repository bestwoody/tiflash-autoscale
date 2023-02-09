package autoscale

import "testing"

func TestMetrics(t *testing.T) {
	InitTestEnv()
	RunAutoscaleHttpServer()
}

package autoscale

import (
	"testing"
)

var (
	addr = "http://127.0.0.1:8081"
)

func TestHttpServer(t *testing.T) {
	IsMockK8s = true
	LogMode = LogModeLocalTest
	InitZapLogger()

	cm := NewClusterManager(EnvRegion, false, nil)
	Cm4Http = cm

	// run http API server
	go RunAutoscaleHttpServer()

	// time.Sleep(3600 * time.Second)
	cm.Wait()
	cm.Shutdown()

	//go InitHTTPTestEnv()

	// test self-metrics
	//resp, err := http.Get(addr + "/self-metrics")
	//assert.NoError(t, err)
	//defer resp.Body.Close()
	//assertEqual(t, resp.StatusCode, http.StatusOK)
	//data, err := io.ReadAll(resp.Body)
	//assert.NoError(t, err)
	//reader := bytes.NewReader(data)
	//var parser expfmt.TextParser
	//metricFamilies, err := parser.TextToMetricFamilies(reader)
	//assert.NoError(t, err)
	//
	//for _, v := range metricFamilies {
	//	if strings.HasPrefix(*v.Name, MetricOf) {
	//		for _, m := range v.Metric {
	//			res += int(*m.Gauge.Value)
	//		}
	//		break
	//	}
	//
	//}

}

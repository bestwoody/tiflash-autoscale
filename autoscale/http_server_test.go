package autoscale

import (
	kruiseclientfake "github.com/openkruise/kruise-api/client/clientset/versioned/fake"
	k8sclientfake "k8s.io/client-go/kubernetes/fake"
	metricclientfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"testing"
)

var (
	addr = "http://127.0.0.1:8081"
)

func InitHTTPTestEnv() {
	LogMode = LogModeLocalTest
	InitZapLogger()
	RunAutoscaleHttpServer()
}

func MockK8sEnv() (config *restclient.Config, K8sCli *k8sclientfake.Clientset, MetricsCli *metricclientfake.Clientset, Cli *kruiseclientfake.Clientset) {
	config = &rest.Config{
		// Set the necessary fields for an in-cluster config
	}
	K8sCli = k8sclientfake.NewSimpleClientset()

	MetricsCli = metricclientfake.NewSimpleClientset()

	Cli = kruiseclientfake.NewSimpleClientset()
	return config, K8sCli, MetricsCli, Cli
}

func TestHttpServer(t *testing.T) {
	MockK8sEnv()
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

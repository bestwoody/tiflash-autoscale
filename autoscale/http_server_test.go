package autoscale

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestHttpServer(t *testing.T) {

	httpServerAddr := "http://127.0.0.1:8081"

	InitTestEnv()
	OptionRunMode = RunModeTest
	IsSupClientMock = true

	cm := NewClusterManager(EnvRegion, false, nil)
	Cm4Http = cm
	readnode1 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "readnode1",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "readnode1", Image: "test", Command: []string{"sleep", "1000"}},
			},
		},
	}
	readnode1.Status.PodIP = "127.0.0.1"

	_, err := cm.K8sCli.CoreV1().Pods(cm.Namespace).Create(context.TODO(), &readnode1, metav1.CreateOptions{})
	assert.NoError(t, err)
	err = cm.AutoScaleMeta.setupManualPauseMockTenant("t2", 1, 4, false, 60, nil)
	assert.NoError(t, err)

	// run http API server
	httpServer := NewAutoscaleHttpServer()
	go httpServer.Run()
	defer httpServer.Close()
	defer cm.Shutdown()

	// wait for http server start
	time.Sleep(15 * time.Second)

	var res map[string]interface{}

	//test promhttp.Handler
	Logger.Infof("[http][test]promhttp.Handler")
	selfMetricsResp, err := http.Get(httpServerAddr + "/self-metrics")
	assert.NoError(t, err)
	defer selfMetricsResp.Body.Close()
	assertEqual(t, selfMetricsResp.StatusCode, http.StatusOK)
	data, err := io.ReadAll(selfMetricsResp.Body)
	assert.NoError(t, err)
	reader := bytes.NewReader(data)
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(reader)
	assert.NoError(t, err)
	assert.True(t, len(metricFamilies) > 0)

	// test GetMetricsFromNode
	Logger.Infof("[http][test]GetMetricsFromNode")
	metricsResp, err := http.PostForm(httpServerAddr+"/metrics", url.Values{
		"node": {""},
	})
	assert.NoError(t, err)
	defer metricsResp.Body.Close()
	assertEqual(t, metricsResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(metricsResp.Body)
	assert.NoError(t, err)
	assertEqual(t, len(data), 0)

	// test GetStateServer
	Logger.Infof("[http][test]GetStateServer")
	getStateResp, err := http.PostForm(httpServerAddr+"/getstate", url.Values{
		"tenantName": {"t2"},
	})
	assert.NoError(t, err)
	defer getStateResp.Body.Close()
	assertEqual(t, getStateResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(getStateResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 0.0)
	assertEqual(t, res["errorInfo"].(string), "")
	assertEqual(t, res["state"].(string), "resumed")
	assertEqual(t, res["numOfRNs"].(float64), 1.0)

	// test HttpHandlePauseForTest
	Logger.Infof("[http][test]HttpHandlePauseForTest")
	pause4testResp, err := http.PostForm(httpServerAddr+"/pause4test", url.Values{
		"tidbclusterid": {"t2"},
	})
	assert.NoError(t, err)
	defer pause4testResp.Body.Close()
	assertEqual(t, pause4testResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(pause4testResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 0.0)
	assertEqual(t, res["errorInfo"].(string), "")
	assertEqual(t, res["state"].(string), "pausing")
	assertEqual(t, res["topology"], nil)

	// wait for pause
	time.Sleep(15 * time.Second)
	getStateResp, err = http.PostForm(httpServerAddr+"/getstate", url.Values{
		"tenantName": {"t2"},
	})
	assert.NoError(t, err)
	defer getStateResp.Body.Close()
	assertEqual(t, getStateResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(getStateResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 0.0)
	assertEqual(t, res["errorInfo"].(string), "")
	assertEqual(t, res["state"].(string), "paused")
	assertEqual(t, res["numOfRNs"].(float64), 0.0)

	// test HttpHandleResumeAndGetTopology
	Logger.Infof("[http][test]HttpHandleResumeAndGetTopology")
	resumeAndGetTopologyResp, err := http.PostForm(httpServerAddr+"/resume-and-get-topology", url.Values{
		"tidbclusterid": {"t2"},
	})
	assert.NoError(t, err)
	defer resumeAndGetTopologyResp.Body.Close()
	assertEqual(t, resumeAndGetTopologyResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(resumeAndGetTopologyResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 0.0)
	assertEqual(t, res["errorInfo"].(string), "")
	assertEqual(t, res["state"].(string), "resumed")
	assertEqual(t, res["topology"].([]interface{})[0].(string), "127.0.0.1:3930")

	// test SharedFixedPool
	Logger.Infof("[http][test]SharedFixedPool")
	shareFixedPoolResp, err := http.Get(httpServerAddr + "/sharedfixedpool")
	assert.NoError(t, err)
	defer shareFixedPoolResp.Body.Close()
	assertEqual(t, shareFixedPoolResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(shareFixedPoolResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 0.0)
	assertEqual(t, res["errorInfo"].(string), "")
	assertEqual(t, res["state"].(string), "fixpool")
	assertEqual(t, len(res["topology"].([]interface{})), 1)
	assertEqual(t, res["topology"].([]interface{})[0].(string), "serverless-cluster-tiflash-cn-0.serverless-cluster-tiflash-cn-peer.tidb-serverless.svc.cluster.local:3930")

	//test DumpMeta
	Logger.Infof("[http][test]DumpMeta")
	dumpMetaResp, err := http.Get(httpServerAddr + "/dumpmeta")
	assert.NoError(t, err)
	defer dumpMetaResp.Body.Close()
	assertEqual(t, dumpMetaResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(dumpMetaResp.Body)
	assert.NoError(t, err)

}

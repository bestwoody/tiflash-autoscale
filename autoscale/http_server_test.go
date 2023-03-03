package autoscale

import (
	"bytes"
	"encoding/json"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

var (
	addr = "http://127.0.0.1:8081"
)

func TestHttpServer(t *testing.T) {
	LogMode = LogModeLocalTest
	OptionRunMode = RunModeLocal
	InitZapLogger()

	cm := NewClusterManager(EnvRegion, false, nil)
	Cm4Http = cm

	// run http API server
	go RunAutoscaleHttpServer()
	defer CloseAutoscaleHttpServer()
	defer cm.Shutdown()

	// wait for http server start
	time.Sleep(5 * time.Second)

	var res map[string]interface{}

	// test SharedFixedPool
	Logger.Infof("[http][test]SharedFixedPool")
	shareFixedPoolResp, err := http.Get(addr + "/sharedfixedpool")
	assert.NoError(t, err)
	defer shareFixedPoolResp.Body.Close()
	assertEqual(t, shareFixedPoolResp.StatusCode, http.StatusOK)
	data, err := io.ReadAll(shareFixedPoolResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 0.0)
	assertEqual(t, res["errorInfo"].(string), "")
	assertEqual(t, res["state"].(string), "fixpool")
	assertEqual(t, res["topology"].([]interface{})[0].(string), "serverless-cluster-tiflash-cn-0.serverless-cluster-tiflash-cn-peer.tidb-serverless.svc.cluster.local:3930")

	//test promhttp.Handler
	Logger.Infof("[http][test]promhttp.Handler")
	selfMetricsResp, err := http.Get(addr + "/self-metrics")
	assert.NoError(t, err)
	defer selfMetricsResp.Body.Close()
	assertEqual(t, selfMetricsResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(selfMetricsResp.Body)
	assert.NoError(t, err)
	reader := bytes.NewReader(data)
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(reader)
	assert.NoError(t, err)
	assert.True(t, len(metricFamilies) > 0)

	// test GetMetricsFromNode
	Logger.Infof("[http][test]GetMetricsFromNode")
	metricsResp, err := http.PostForm(addr+"/metrics", url.Values{
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
	getStateResp, err := http.PostForm(addr+"/getstate", url.Values{
		"tenantName": {"t1"},
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

	// test HttpHandlePauseForTest
	Logger.Infof("[http][test]HttpHandlePauseForTest")
	pause4testResp, err := http.PostForm(addr+"/pause4test", url.Values{
		"tidbclusterid": {"t1"},
	})
	assert.NoError(t, err)
	defer pause4testResp.Body.Close()
	assertEqual(t, pause4testResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(pause4testResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 1.0)
	assertEqual(t, res["errorInfo"].(string), "pause failed")
	assertEqual(t, res["state"].(string), "paused")
	assertEqual(t, res["topology"], nil)

	// test HttpHandleResumeAndGetTopology
	Logger.Infof("[http][test]HttpHandleResumeAndGetTopology")
	resumeAndGetTopologyResp, err := http.PostForm(addr+"/resume-and-get-topology", url.Values{
		"tidbclusterid": {"t1"},
	})
	assert.NoError(t, err)
	defer resumeAndGetTopologyResp.Body.Close()
	assertEqual(t, resumeAndGetTopologyResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(resumeAndGetTopologyResp.Body)
	assert.NoError(t, err)
	err = json.Unmarshal(data, &res)
	assert.NoError(t, err)
	assertEqual(t, res["hasError"].(float64), 1.0)
	assertEqual(t, res["errorInfo"].(string), "resume failed")
	assertEqual(t, res["state"].(string), "resumed")
	assertEqual(t, len(res["topology"].([]interface{})), 0)

	//test DumpMeta
	Logger.Infof("[http][test]DumpMeta")
	dumpMetaResp, err := http.Get(addr + "/dumpmeta")
	assert.NoError(t, err)
	defer dumpMetaResp.Body.Close()
	assertEqual(t, dumpMetaResp.StatusCode, http.StatusOK)
	data, err = io.ReadAll(dumpMetaResp.Body)
	assert.NoError(t, err)
	print(string(data))

}

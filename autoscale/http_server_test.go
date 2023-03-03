package autoscale

import (
	"bytes"
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
	IsMockK8s = true
	LogMode = LogModeLocalTest
	InitZapLogger()

	cm := NewClusterManager(EnvRegion, false, nil)
	Cm4Http = cm

	// run http API server
	go RunAutoscaleHttpServer()
	defer CloseAutoscaleHttpServer()
	defer cm.Shutdown()

	// wait for http server start
	time.Sleep(5 * time.Second)

	//test promhttp.Handler
	Logger.Infof("[http][test]promhttp.Handler")
	resp1, err := http.Get(addr + "/self-metrics")
	assert.NoError(t, err)
	defer resp1.Body.Close()
	assertEqual(t, resp1.StatusCode, http.StatusOK)
	data, err := io.ReadAll(resp1.Body)
	assert.NoError(t, err)
	reader := bytes.NewReader(data)
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(reader)
	assert.NoError(t, err)
	assert.True(t, len(metricFamilies) > 0)

	// test GetMetricsFromNode
	Logger.Infof("[http][test]GetMetricsFromNode")
	resp2, err := http.PostForm(addr+"/metrics", url.Values{
		"node": {""},
	})
	assert.NoError(t, err)
	defer resp2.Body.Close()
	assertEqual(t, resp2.StatusCode, http.StatusOK)
	data, err = io.ReadAll(resp2.Body)
	assert.NoError(t, err)
	assertEqual(t, len(data), 0)

	// test GetStateServer
	Logger.Infof("[http][test]GetStateServer")
	resp3, err := http.PostForm(addr+"/getstate", url.Values{
		"tenantName": {""},
	})
	assert.NoError(t, err)
	defer resp3.Body.Close()
	assertEqual(t, resp3.StatusCode, http.StatusOK)
	data, err = io.ReadAll(resp3.Body)
	assert.NoError(t, err)
}

package autoscale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	rest "k8s.io/client-go/rest"
)

var HttpResumeWaitTimoueSec = 30
var HttpResumeCheckIntervalMs = 100

var httpReqId = atomic.Int32{}

type ResumeAndGetTopologyResult struct {
	HasError  int      `json:"hasError"`
	ErrorInfo string   `json:"errorInfo"`
	State     string   `json:"state"`
	Topology  []string `json:"topology"`
	Timestamp string   `json:"timestamp"`
}

type GetStateResult struct {
	HasError  int    `json:"hasError"`
	ErrorInfo string `json:"errorInfo"`
	State     string `json:"state"`
	NumOfRNs  int    `json:"numOfRNs"`
}

func (ret *GetStateResult) WriteResp(hasErr int, errInfo string, state string, numOfRNs int) []byte {
	ret.HasError = hasErr
	ret.ErrorInfo = errInfo
	ret.State = state
	ret.NumOfRNs = numOfRNs
	retJson, _ := json.Marshal(ret)
	return retJson
}

const (
	TenantStateResumedString  = "resumed"
	TenantStateResumingString = "resuming"
	TenantStatePausedString   = "paused"
	TenantStatePausingString  = "pausing"
	TenantStateUnknownString  = "unknown"
	HttpServerPort            = "8081"
)

var (
	Cm4Http *ClusterManager
)

type AutoscaleHttpServer struct {
	server *http.Server
}

func getIP(r *http.Request) (string, error) {
	ips := r.Header.Get("X-Forwarded-For")
	splitIps := strings.Split(ips, ",")

	if len(splitIps) > 0 {
		// get last IP in list since ELB prepends other user defined IPs, meaning the last one is the actual client IP.
		netIP := net.ParseIP(splitIps[len(splitIps)-1])
		if netIP != nil {
			return netIP.String(), nil
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	netIP := net.ParseIP(ip)
	if netIP != nil {
		ip := netIP.String()
		if ip == "::1" {
			return "127.0.0.1", nil
		}
		return ip, nil
	}

	return "", errors.New("IP not found")
}

func (ret *ResumeAndGetTopologyResult) WriteResp(hasErr int, state string, errInfo string, topo []string) []byte {
	// Logger.Infof("[HTTP]ResumeAndGetTopology, WriteResp: %v | %v | %v | %+v", hasErr, state, errInfo, topo)
	ret.HasError = hasErr
	ret.State = state
	ret.ErrorInfo = errInfo
	ret.Topology = topo
	ret.Timestamp = strconv.FormatInt(time.Now().UnixNano(), 10)
	retJson, _ := json.Marshal(ret)
	return retJson
}

func SharedFixedPool(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() {
		MetricOfHttpRequestSharedFixedPoolMetricSeconds.Observe(time.Since(start).Seconds())
	}()
	MetricOfHttpRequestSharedFixedPoolCnt.Inc()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	ip, _ := getIP(req)
	Logger.Infof("[HTTP]SharedFixedPool, client: %v", ip)
	ret := ResumeAndGetTopologyResult{Topology: make([]string, 0, 5)}
	if UseSpecialTenantAsFixPool {
		io.WriteString(w, string(ret.WriteResp(0, "fixpool", "", Cm4Http.AutoScaleMeta.GetTopology(SpecialTenantNameForFixPool))))
	} else {
		fixCNs := make([]string, 0, 2)
		replica := int(Cm4Http.ExternalFixPoolReplica.Load())
		for i := 0; i < replica; i++ {
			fixCNs = append(fixCNs, fmt.Sprintf("serverless-cluster-tiflash-cn-%v.serverless-cluster-tiflash-cn-peer.tidb-serverless.svc.cluster.local:3930", i))
		}
		io.WriteString(w, string(ret.WriteResp(0, "fixpool", "", fixCNs)))
	}
}

func ResumeAndGetTopology(w http.ResponseWriter, tenantName string, reqid int32, version string) {
	ret := ResumeAndGetTopologyResult{Topology: make([]string, 0, 5)}
	retStr := ""
	defer func() {
		Logger.Infof("[HTTP]ResumeAndGetTopology, reqid:%v resp:%v", reqid, retStr)
	}()
	if tenantName == "" {
		retStr = string(ret.WriteResp(1, "unknown", "invalid tidbclusterid", nil))
		io.WriteString(w, retStr)
		return
	}
	flag, currentState, _ := Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
	if !flag {
		//register new tenant for serveless tier
		if OptionRunMode == RunModeLocal || OptionRunMode == RunModeServeless || OptionRunMode == RunModeDedicated || OptionRunMode == RunModeTest {
			Cm4Http.AutoScaleMeta.SetupAutoPauseTenantWithPausedState(tenantName, DefaultMinCntOfPod, DefaultMaxCntOfPod, version)
		}
		flag, currentState, _ = Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
		if !flag {
			retStr = string(ret.WriteResp(1, TenantState2String(currentState), "get state failed, tenant does not exist", nil))
			io.WriteString(w, retStr)
			return
		}
	}
	tenantDesc := Cm4Http.AutoScaleMeta.GetTenantDesc(tenantName)
	tenantDesc.CheckAndUpdateVersion(version)
	// state := req.FormValue("state")

	// if currentState == TenantStatePaused {
	flag = Cm4Http.Resume(tenantName)
	_, currentState, _ = Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
	// tenantDesc := Cm4Http.AutoScaleMeta.GetTenantDesc(tenantName)
	minCntOfRequiredPods := 1
	// TODO revert
	// minCntOfRequiredPods := DefaultMinCntOfPod
	// if tenantDesc != nil {
	// 	minCntOfRequiredPods = tenantDesc.GetMinCntOfPod()
	// }

	// wait util topology is not empty or timeout
	if len(Cm4Http.AutoScaleMeta.GetTopology(tenantName)) < minCntOfRequiredPods {
		Logger.Warnf("[HTTP]ResumeAndGetTopology, resumed but topology is not ready, begin to wait at most %vs", HttpResumeWaitTimoueSec)
		waitSt := time.Now()
		topo := Cm4Http.AutoScaleMeta.GetTopology(tenantName)
		for len(topo) < minCntOfRequiredPods && time.Since(waitSt).Seconds() < float64(HttpResumeWaitTimoueSec) {
			// for time.Now().UnixMilli()-waitSt.UnixMilli() < 15*1000 {
			time.Sleep(time.Duration(HttpResumeCheckIntervalMs) * time.Millisecond)
			// Logger.Warnf("[HTTP]ResumeAndGetTopology, resumed and topology is not ready, keep waiting, it has cost %vms, topo: %+v", time.Since(waitSt).Milliseconds(), topo)
			flag = Cm4Http.Resume(tenantName)
			topo = Cm4Http.AutoScaleMeta.GetTopology(tenantName)
		}
		Logger.Warnf("[HTTP]ResumeAndGetTopology, resumed and topology is ready, tenant: %v , wait cost %vms", tenantName, time.Since(waitSt).Milliseconds())
	}

	if len(Cm4Http.AutoScaleMeta.GetTopology(tenantName)) <= 0 {
		Logger.Errorf("[HTTP]ResumeAndGetTopology, wait topology ready timeout: %vs!! ", HttpResumeWaitTimoueSec)
	}

	if !flag {
		retStr = string(ret.WriteResp(1, TenantState2String(currentState), "resume failed", Cm4Http.AutoScaleMeta.GetTopology(tenantName)))
		io.WriteString(w, retStr)
		return
	} else {
		retStr = string(ret.WriteResp(0, TenantState2String(currentState), "", Cm4Http.AutoScaleMeta.GetTopology(tenantName)))
		io.WriteString(w, retStr)
		return
	}
	// } else {
	// 	io.WriteString(w, string(ret.WriteResp(1, TenantState2String(currentState), "unnecessary to resume, ComputePool state is not paused", Cm4Http.AutoScaleMeta.GetTopology(tenantName))))
	// }
}

func HttpHandleResumeAndGetTopology(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	curReqID := httpReqId.Add(1)
	defer func() {
		MetricOfHttpRequestHttpHandleResumeAndGetTopologyMetricSeconds.Observe(time.Since(start).Seconds())
	}()
	MetricOfHttpRequestHttpHandleResumeAndGetTopologyCnt.Inc()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	ip, _ := getIP(req)
	tenantName := req.FormValue("tidbclusterid")
	version := req.FormValue("cn_version")
	Logger.Infof("[HTTP]ResumeAndGetTopology, tenantName: %v, client: %v, reqid: %v, cn_version: %v", tenantName, ip, curReqID, version)
	ResumeAndGetTopology(w, tenantName, curReqID, version)
}

func HttpHandlePauseForTest(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() {
		MetricOfHttpRequestHttpHandlePauseForTestMetricSeconds.Observe(time.Since(start).Seconds())
	}()
	MetricOfHttpRequestHttpHandlePauseForTestCnt.Inc()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	tenantName := req.FormValue("tidbclusterid")

	ret := ResumeAndGetTopologyResult{Topology: make([]string, 0, 5)}
	if tenantName == "" {
		io.WriteString(w, string(ret.WriteResp(1, "unknown", "invalid tidbclusterid", nil)))
		return
	}

	// state := req.FormValue("state")
	ip, _ := getIP(req)
	Logger.Infof("[HTTP]ResumeAndGetTopology, tenantName: %v, client: %v", tenantName, ip)
	// if currentState == TenantStatePaused {
	flag := Cm4Http.AsyncPause(tenantName)
	_, currentState, _ := Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
	if !flag {
		io.WriteString(w, string(ret.WriteResp(1, TenantState2String(currentState), "pause failed", nil)))
		return
	} else {
		io.WriteString(w, string(ret.WriteResp(0, TenantState2String(currentState), "", nil)))
		return
	}
}

func DumpMeta(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() {
		MetricOfHttpRequestDumpMetaSeconds.Observe(time.Since(start).Seconds())
	}()
	MetricOfHttpRequestDumpMetaCnt.Inc()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	Logger.Infof("[http]req of DumpMeta")
	io.WriteString(w, Cm4Http.AutoScaleMeta.Dump())
}

func GetStateServer(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() {
		MetricOfHttpRequestGetStateServerSeconds.Observe(time.Since(start).Seconds())
	}()
	MetricOfHttpRequestGetStateServerCnt.Inc()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	tenantName := req.FormValue("tenantName")
	if tenantName == "" {
		tenantName = "t1"
	}
	ret := GetStateResult{}
	flag, state, numOfRNs := Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
	if !flag {
		io.WriteString(w, string(ret.WriteResp(1, "get state failed", TenantState2String(state), numOfRNs)))
		return
	}
	retJsonStr := string(ret.WriteResp(0, "", TenantState2String(state), numOfRNs))
	Logger.Infof("[http]resp of getstate, '%v' ", retJsonStr)
	io.WriteString(w, retJsonStr)
}

func addLabel(metric *dto.Metric, extraLabelName string, extraLabelVal string) {
	metric.Label = append(metric.Label, &dto.LabelPair{
		Name:  &extraLabelName,
		Value: &extraLabelVal,
	})
}

func proxyMetrics(restCli rest.Interface, node string, podDescMap map[string]*PodDesc) (string, error) {
	data, err := restCli.Get().AbsPath(fmt.Sprintf("/api/v1/nodes/%v/proxy/metrics/cadvisor", node)).DoRaw(context.TODO())
	if err != nil {
		return "", err
	}
	reader := bytes.NewReader(data)
	// fmt.Println(string(data))
	out := bytes.NewBuffer(make([]byte, 0, 102400))
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return "", err
	}
	for _, v := range metricFamilies {

		// fmt.Println("###MF_NAME: " + *v.Name)
		// if *v.Name != "container_cpu_usage_seconds_total" {
		// 	continue
		// }
		// metrics :=
		for _, metric := range v.GetMetric() {

			labels := metric.Label
			// Logger.Infof("##LABELS_LEN %v", (len(labels)))

			// extraLabelName := "tenant"
			// extraLabelVal := "T1"
			// metric.Label = append(metric.Label, &dto.LabelPair{
			// 	Name:  &extraLabelName,
			// 	Value: &extraLabelVal,
			// })
			podName := ""
			for _, label := range labels {
				if *label.Name == "pod" && *label.Value != "" {
					// fmt.Println("pod: " + *label.Value)
					podName = *label.Value
				}

				// fmt.Println("##LABEL " + *label.Name + ": " + *label.Value)
			}
			if podName != "" {
				v, ok := podDescMap[podName]
				if ok {
					addLabel(metric, "metrics_topic", "cadvisor")
					addLabel(metric, "metrics_source", "compute_pod")
					addLabel(metric, "pod_ip", v.IP)
					addLabel(metric, "tidb_cluster", v.GetTenantName())
				}
			}
			// fmt.Println(metric.Label)
		}
		// fmt.Println(k + ":" + v.String())
		out.WriteString(("\n"))
		expfmt.MetricFamilyToText(out, v)
		// fmt.Println(out.String())
	}
	return out.String(), nil
}

func GetMetricsFromNode(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer func() {
		MetricOfHttpRequestGetMetricsFromNodeSeconds.Observe(time.Since(start).Seconds())
	}()
	MetricOfHttpRequestGetMetricsFromNodeCnt.Inc()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	node := req.FormValue("node")
	Logger.Infof("[http]GetMetricsFromNode, node:%v", node)
	if node == "" {
		return
	}

	resp, err := proxyMetrics(Cm4Http.AutoScaleMeta.k8sCli.RESTClient(), node, Cm4Http.AutoScaleMeta.CopyPodDescMap()) //Cm4Http.AutoScaleMeta.PodDescMap
	if err != nil {
		Logger.Errorf("[error]GetMetricsFromNode failed, node: %v err: %v", node, err.Error())
		return
	}
	io.WriteString(w, resp)
}

func NewAutoscaleHttpServer() *AutoscaleHttpServer {
	Logger.Infof("[http]Access-Control-Allow-Origin is enabled")
	// autoscale.HardCodeEnvPdAddr = os.Getenv("PD_ADDR")
	// autoscale.HardCodeEnvTidbStatusAddr = os.Getenv("TIDB_STATUS_ADDR")
	// Logger.Infof("env.PD_ADDR: %v", autoscale.HardCodeEnvPdAddr)
	// Logger.Infof("env.TIDB_STATUS_ADDR: %v", autoscale.HardCodeEnvTidbStatusAddr)
	// Cm4Http = autoscale.NewClusterManager()

	http.HandleFunc("/getstate", GetStateServer)
	http.HandleFunc("/metrics", GetMetricsFromNode)
	http.Handle("/self-metrics", promhttp.Handler())
	http.HandleFunc("/resume-and-get-topology", HttpHandleResumeAndGetTopology)
	http.HandleFunc("/pause4test", HttpHandlePauseForTest)
	http.HandleFunc("/sharedfixedpool", SharedFixedPool)
	http.HandleFunc("/dumpmeta", DumpMeta)
	srv := &http.Server{Addr: ":" + HttpServerPort}
	ret := &AutoscaleHttpServer{
		server: srv,
	}
	return ret
}

func (cur *AutoscaleHttpServer) Run() {
	Logger.Infof("[HTTP]ListenAndServe " + HttpServerPort)
	err := cur.server.ListenAndServe()
	if err == http.ErrServerClosed {
		Logger.Infof("[HTTP]server closed")
		err = nil
	}
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func (cur *AutoscaleHttpServer) Close() {
	if cur.server == nil {
		return
	}
	Logger.Infof("[HTTP]Prepare to shut down the server")
	cur.server.Shutdown(context.Background())
}

package autoscale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	rest "k8s.io/client-go/rest"
)

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
)

var (
	Cm4Http *ClusterManager
)

func (ret *ResumeAndGetTopologyResult) WriteResp(hasErr int, state string, errInfo string, topo []string) []byte {
	ret.HasError = hasErr
	ret.State = state
	ret.ErrorInfo = errInfo
	ret.Topology = topo
	ret.Timestamp = strconv.FormatInt(time.Now().UnixNano(), 10)
	retJson, _ := json.Marshal(ret)
	return retJson
}

func SharedFixedPool(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	Logger.Infof("[HTTP]SharedFixedPool")
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

func ResumeAndGetTopology(w http.ResponseWriter, tenantName string) {
	ret := ResumeAndGetTopologyResult{Topology: make([]string, 0, 5)}
	if tenantName == "" {
		io.WriteString(w, string(ret.WriteResp(1, "unknown", "invalid tidbclusterid", nil)))
		return
	}
	flag, currentState, _ := Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
	if !flag {
		//register new tenant for serveless tier
		if OptionRunMode == RunModeLocal || OptionRunMode == RunModeServeless {
			Cm4Http.AutoScaleMeta.SetupAutoPauseTenantWithPausedState(tenantName, DefaultMinCntOfPod, DefaultMaxCntOfPod)
		}
		flag, currentState, _ = Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
		if !flag {
			io.WriteString(w, string(ret.WriteResp(1, TenantState2String(currentState), "get state failed, tenant does not exist", nil)))
			return
		}
	}
	// state := req.FormValue("state")
	Logger.Infof("[HTTP]ResumeAndGetTopology, tenantName: %v", tenantName)
	if currentState == TenantStatePaused {
		flag = Cm4Http.Resume(tenantName)
		_, currentState, _ = Cm4Http.AutoScaleMeta.GetTenantState(tenantName)

		if !flag {
			io.WriteString(w, string(ret.WriteResp(1, TenantState2String(currentState), "resume failed", Cm4Http.AutoScaleMeta.GetTopology(tenantName))))
			return
		} else {
			io.WriteString(w, string(ret.WriteResp(0, TenantState2String(currentState), "", Cm4Http.AutoScaleMeta.GetTopology(tenantName))))
			return
		}
	} else {
		io.WriteString(w, string(ret.WriteResp(1, TenantState2String(currentState), "unnecessary to resume, ComputePool state is not paused", Cm4Http.AutoScaleMeta.GetTopology(tenantName))))
	}
}

func HttpHandleResumeAndGetTopology(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	tenantName := req.FormValue("tidbclusterid")
	ResumeAndGetTopology(w, tenantName)
}

func DumpMeta(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	Logger.Infof("[http]req of DumpMeta")
	io.WriteString(w, Cm4Http.AutoScaleMeta.Dump())
}

func GetStateServer(w http.ResponseWriter, req *http.Request) {
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

func RunAutoscaleHttpServer() {
	Logger.Infof("[http]Access-Control-Allow-Origin is enabled")
	// autoscale.HardCodeEnvPdAddr = os.Getenv("PD_ADDR")
	// autoscale.HardCodeEnvTidbStatusAddr = os.Getenv("TIDB_STATUS_ADDR")
	// Logger.Infof("env.PD_ADDR: %v", autoscale.HardCodeEnvPdAddr)
	// Logger.Infof("env.TIDB_STATUS_ADDR: %v", autoscale.HardCodeEnvTidbStatusAddr)
	// Cm4Http = autoscale.NewClusterManager()

	http.HandleFunc("/getstate", GetStateServer)
	http.HandleFunc("/metrics", GetMetricsFromNode)
	http.HandleFunc("/resume-and-get-topology", HttpHandleResumeAndGetTopology)
	http.HandleFunc("/sharedfixedpool", SharedFixedPool)
	http.HandleFunc("/dumpmeta", DumpMeta)

	Logger.Infof("[HTTP]ListenAndServe 8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

package autoscale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	rest "k8s.io/client-go/rest"
)

type SetStateResult struct {
	HasError  int    `json:"hasError"`
	ErrorInfo string `json:"errorInfo"`
	State     string `json:"state"`
}

type GetStateResult struct {
	HasError  int    `json:"hasError"`
	ErrorInfo string `json:"errorInfo"`
	State     string `json:"state"`
	NumOfRNs  int    `json:"numOfRNs"`
}

const (
	TenantStateResumedString  = "available"
	TenantStateResumingString = "resuming"
	TenantStatePausedString   = "paused"
	TenantStatePausingString  = "pausing"
)

var (
	Cm4Http *ClusterManager
)

func SetStateServer(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	tenantName := req.FormValue("tenantName")
	ret := SetStateResult{}
	if tenantName == "" {
		tenantName = "t1"
	}
	flag, currentState, _ := Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
	if !flag {
		ret.HasError = 1
		ret.ErrorInfo = "get state failed"
		retJson, _ := json.Marshal(ret)
		io.WriteString(w, string(retJson))
		return
	}
	state := req.FormValue("state")
	Logger.Infof("[HTTP]SetStateServer, state: %v", state)
	if currentState == TenantStatePaused && state == "resume" {
		flag = Cm4Http.Resume(tenantName)
		if !flag {
			ret.HasError = 1
			ret.ErrorInfo = "resume failed"
			retJson, _ := json.Marshal(ret)
			io.WriteString(w, string(retJson))
			return
		}
		_, currentState, _ = Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
		ret.State = ConvertStateString(currentState)
		retJson, _ := json.Marshal(ret)
		io.WriteString(w, string(retJson))
		return
	} else if currentState == TenantStateResumed && state == "pause" {
		flag = Cm4Http.Pause(tenantName)
		if !flag {
			ret.HasError = 1
			ret.ErrorInfo = "pause failed"
			retJson, _ := json.Marshal(ret)
			io.WriteString(w, string(retJson))
			return
		}
		_, currentState, _ = Cm4Http.AutoScaleMeta.GetTenantState(tenantName)
		ret.State = ConvertStateString(currentState)
		retJson, _ := json.Marshal(ret)
		io.WriteString(w, string(retJson))
		return
	}
	ret.HasError = 1
	ret.State = ConvertStateString(currentState)
	ret.ErrorInfo = "invalid set state"
	retJson, _ := json.Marshal(ret)
	io.WriteString(w, string(retJson))
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
		ret.HasError = 1
		ret.ErrorInfo = "get state failed"
		retJson, _ := json.Marshal(ret)
		io.WriteString(w, string(retJson))
		return
	}
	ret.NumOfRNs = numOfRNs
	ret.State = ConvertStateString(state)
	retJson, _ := json.Marshal(ret)
	retJsonStr := string(retJson)
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
					addLabel(metric, "tidb_cluster", v.TenantName)
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

func ConvertStateString(state int32) string {
	if state == TenantStateResumed {
		return TenantStateResumedString
	} else if state == TenantStateResuming {
		return TenantStateResumingString
	} else if state == TenantStatePaused {
		return TenantStatePausedString
	}
	return TenantStatePausingString
}

func RunAutoscaleHttpServer() {
	Logger.Infof("[http]Access-Control-Allow-Origin is enabled")
	// autoscale.HardCodeEnvPdAddr = os.Getenv("PD_ADDR")
	// autoscale.HardCodeEnvTidbStatusAddr = os.Getenv("TIDB_STATUS_ADDR")
	// Logger.Infof("env.PD_ADDR: %v", autoscale.HardCodeEnvPdAddr)
	// Logger.Infof("env.TIDB_STATUS_ADDR: %v", autoscale.HardCodeEnvTidbStatusAddr)
	// Cm4Http = autoscale.NewClusterManager()

	http.HandleFunc("/setstate", SetStateServer)
	http.HandleFunc("/getstate", GetStateServer)
	http.HandleFunc("/metrics", GetMetricsFromNode)
	http.HandleFunc("/dumpmeta", DumpMeta)

	Logger.Infof("[HTTP]ListenAndServe 8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

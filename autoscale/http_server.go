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
	log.Printf("[HTTP]SetStateServer, state: %v\n", state)
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
		ret.State = convertStateString(currentState)
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
		ret.State = convertStateString(currentState)
		retJson, _ := json.Marshal(ret)
		io.WriteString(w, string(retJson))
		return
	}
	ret.HasError = 1
	ret.State = convertStateString(currentState)
	ret.ErrorInfo = "invalid set state"
	retJson, _ := json.Marshal(ret)
	io.WriteString(w, string(retJson))
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
	ret.State = convertStateString(state)
	retJson, _ := json.Marshal(ret)
	retJsonStr := string(retJson)
	log.Printf("[http]resp of getstate, '%v' \n", retJsonStr)
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
			// fmt.Printf("##LABELS_LEN %v\n", (len(labels)))

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
		out.WriteString("\n")
		expfmt.MetricFamilyToText(out, v)
		// fmt.Println(out.String())
	}
	return out.String(), nil
}

func GetMetricsFromNode(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	node := req.FormValue("node")
	log.Printf("[info][http]GetMetricsFromNode, node:%v\n", node)
	if node == "" {
		return
	}

	resp, err := proxyMetrics(Cm4Http.AutoScaleMeta.k8sCli.RESTClient(), node, Cm4Http.AutoScaleMeta.CopyPodDescMap()) //Cm4Http.AutoScaleMeta.PodDescMap
	if err != nil {
		log.Printf("[error]GetMetricsFromNode failed, node: %v err: %v\n", node, err.Error())
		return
	}
	io.WriteString(w, resp)
}

func convertStateString(state int32) string {
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
	log.Printf("[http]Access-Control-Allow-Origin is enabled\n")
	// autoscale.HardCodeEnvPdAddr = os.Getenv("PD_ADDR")
	// autoscale.HardCodeEnvTidbStatusAddr = os.Getenv("TIDB_STATUS_ADDR")
	// fmt.Printf("env.PD_ADDR: %v\n", autoscale.HardCodeEnvPdAddr)
	// fmt.Printf("env.TIDB_STATUS_ADDR: %v\n", autoscale.HardCodeEnvTidbStatusAddr)
	// Cm4Http = autoscale.NewClusterManager()

	http.HandleFunc("/setstate", SetStateServer)
	http.HandleFunc("/getstate", GetStateServer)
	http.HandleFunc("/metrics", GetMetricsFromNode)
	log.Printf("[HTTP]ListenAndServe 8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

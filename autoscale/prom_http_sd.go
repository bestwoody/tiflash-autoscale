package autoscale

type Seller struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code"`
}
type Product struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Seller Seller `json:"seller"`
	Price  int    `json:"price"`
}

type PromTargets struct {
	Host string `json:"host"`
}

type PromJob struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

type HttpSdResp struct {
	PromJobs []PromJob
}

type MetricsCollectInfoOfPod struct {
	labelPodName     string // "pod_name"
	labelPodIP       string // "pod_ip"
	labelNodeIP      string // "node_ip"
	labelTidbCluster string // "tidb_cluster"
	// tiflash metric_port: 8234
}

type PodsToCollectMetrics struct {
	PodMap map[string]MetricsCollectInfoOfPod
}

func (c *PodsToCollectMetrics) ToJson() {

}

package autoscale

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	tenantCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_tenant_count",
		Help: "The current number of tenants",
	})

	podCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_pod_count",
		Help: "The current number of pods",
	})

	rpcRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_rpc_request_total",
			Help: "The total number of rpc requests",
		},
		[]string{"function"},
	)

	rpcRequestMilliSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "autoscale_rpc_request_milliseconds",
			Help: "The duration of rpc requests",
		},
		[]string{"function"},
	)

	httpRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_http_request_total",
			Help: "The total number of http requests",
		},
		[]string{"function"},
	)

	httpRequestMilliSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "autoscale_http_request_milliseconds",
			Help: "The duration of http requests",
		},
		[]string{"function"},
	)
)

func RegisterMetrics() {
	prometheus.MustRegister(tenantCount)
	prometheus.MustRegister(podCount)
	prometheus.MustRegister(rpcRequestTotal)
	prometheus.MustRegister(rpcRequestMilliSeconds)
	prometheus.MustRegister(httpRequestTotal)
	prometheus.MustRegister(httpRequestMilliSeconds)
}

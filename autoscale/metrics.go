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

	rpcRequestTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "autoscale_rpc_request_total",
		Help: "The total number of rpc requests",
	})

	rpcRequestMilliSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "autoscale_rpc_request_milliseconds",
		Help: "The duration of rpc requests",
	})

	httpRequestTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "autoscale_http_request_total",
		Help: "The total number of http requests",
	})

	httpRequestMilliSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "autoscale_http_request_milliseconds",
		Help: "The duration of http requests",
	})
)

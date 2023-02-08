package autoscale

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TenantCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_tenant_count",
		Help: "The current number of tenants",
	})

	PodCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_pod_count",
		Help: "The current number of pods",
	})

	DoPodsWarmCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "autoscale_do_pods_warm_count",
			Help: "The current number of pods in function DoPodsWarm",
		}, []string{"type"},
	)

	WatchPodsLoopEventTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_watch_pods_loop_event_count",
			Help: "The total number of events in function WatchPodsLoop",
		}, []string{"type"},
	)

	RpcRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_rpc_request_total",
			Help: "The total number of rpc requests",
		},
		[]string{"function"},
	)

	RpcRequestMS = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "autoscale_rpc_request_ms",
			Help: "The duration of rpc requests",
		},
		[]string{"function"},
	)

	HttpRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_http_request_total",
			Help: "The total number of http requests",
		},
		[]string{"function"},
	)

	HttpRequestMS = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "autoscale_http_request_ms",
			Help: "The duration of http requests",
		},
		[]string{"function"},
	)

	AddPodIntoTenantTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_add_pod_into_tenant_total",
			Help: "The total number of requesting function AddPodIntoTenant",
		},
	)

	AddPodIntoTenantMS = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name: "autoscale_add_pod_into_tenant_ms",
			Help: "The duration of requesting function AddPodIntoTenant",
		},
	)

	AddPodTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_add_pod_total",
			Help: "The total number of added pods",
		},
	)

	RemovePodFromTenantTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_remove_pod_from_tenant_total",
			Help: "The total number of requesting function RemovePodFromTenant",
		},
	)

	RemovePodFromTenantMS = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name: "autoscale_remove_pod_from_tenant_ms",
			Help: "The duration of requesting function RemovePodFromTenant",
		},
	)

	RemovePodTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_remove_pod_total",
			Help: "The total number of removed pods",
		},
	)

	SupervisorClientRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_request_total",
			Help: "The total number of requests to supervisor",
		},
		[]string{"function"},
	)

	SupervisorClientRequestMS = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "autoscale_supervisor_client_request_ms",
			Help: "The duration of requests to supervisor",
		},
		[]string{"function"},
	)
)

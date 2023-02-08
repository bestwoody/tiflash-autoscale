package autoscale

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TenantCountMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_tenant_count",
		Help: "The current number of tenants",
	})

	PodCountMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_pod_count",
		Help: "The current number of pods",
	})

	DoPodsWarmCountMetric = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "autoscale_do_pods_warm_count",
			Help: "The current number of pods in function DoPodsWarm",
		}, []string{"type"},
	)

	DoPodsWarmCountFailMetric    = DoPodsWarmCountMetric.WithLabelValues("fail")
	DoPodsWarmCountDeltaMetric   = DoPodsWarmCountMetric.WithLabelValues("delta")
	DoPodsWarmCountPendingMetric = DoPodsWarmCountMetric.WithLabelValues("pending")
	DoPodsWarmCountValidMetric   = DoPodsWarmCountMetric.WithLabelValues("valid")

	WatchPodsLoopEventTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_watch_pods_loop_event_total",
			Help: "The total number of events in function WatchPodsLoop",
		}, []string{"type"},
	)

	WatchPodsLoopEventTotalAddedMetric    = WatchPodsLoopEventTotalMetric.WithLabelValues("added")
	WatchPodsLoopEventTotalModifiedMetric = WatchPodsLoopEventTotalMetric.WithLabelValues("modified")
	WatchPodsLoopEventTotalDeletedMetric  = WatchPodsLoopEventTotalMetric.WithLabelValues("deleted")
	WatchPodsLoopEventTotalErrorMetric    = WatchPodsLoopEventTotalMetric.WithLabelValues("error")
	WatchPodsLoopEventTotalBookmarkMetric = WatchPodsLoopEventTotalMetric.WithLabelValues("bookmark")

	RpcRequestTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_rpc_request_total",
			Help: "The total number of rpc requests",
		},
		[]string{"function"},
	)

	RpcRequestTotalResumeAndGetTopologyMetric = RpcRequestTotalMetric.WithLabelValues("ResumeAndGetTopology")
	RpcRequestTotalGetTopologyMetric          = RpcRequestTotalMetric.WithLabelValues("GetTopology")

	RpcRequestSecondsMetric = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_rpc_request_seconds",
			Help:    "The duration of rpc requests",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"function"},
	)

	RpcRequestSecondsResumeAndGetTopologyMetric = RpcRequestSecondsMetric.WithLabelValues("ResumeAndGetTopology")
	RpcRequestSecondsGetTopologyMetric          = RpcRequestSecondsMetric.WithLabelValues("GetTopology")

	HttpRequestTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_http_request_total",
			Help: "The total number of http requests",
		},
		[]string{"function"},
	)

	HttpRequestTotalGetStateServerMetric                 = HttpRequestTotalMetric.WithLabelValues("GetStateServer")
	HttpRequestTotalSharedFixedPoolMetric                = HttpRequestTotalMetric.WithLabelValues("SharedFixedPool")
	HttpRequestTotalGetMetricsFromNodeMetric             = HttpRequestTotalMetric.WithLabelValues("GetMetricsFromNode")
	HttpRequestTotalHttpHandleResumeAndGetTopologyMetric = HttpRequestTotalMetric.WithLabelValues("HttpHandleResumeAndGetTopology")
	HttpRequestTotalHttpHandlePauseForTestMetric         = HttpRequestTotalMetric.WithLabelValues("HttpHandlePauseForTest")
	HttpRequestTotalDumpMetaMetric                       = HttpRequestTotalMetric.WithLabelValues("DumpMeta")

	HttpRequestSecondsMetric = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_http_request_seconds",
			Help:    "The duration of http requests",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"function"},
	)

	HttpRequestSecondsGetStateServerMetric                 = HttpRequestSecondsMetric.WithLabelValues("GetStateServer")
	HttpRequestSecondsSharedFixedPoolMetric                = HttpRequestSecondsMetric.WithLabelValues("SharedFixedPool")
	HttpRequestSecondsGetMetricsFromNodeMetric             = HttpRequestSecondsMetric.WithLabelValues("GetMetricsFromNode")
	HttpRequestSecondsHttpHandleResumeAndGetTopologyMetric = HttpRequestSecondsMetric.WithLabelValues("HttpHandleResumeAndGetTopology")
	HttpRequestSecondsHttpHandlePauseForTestMetric         = HttpRequestSecondsMetric.WithLabelValues("HttpHandlePauseForTest")
	HttpRequestSecondsDumpMetaMetric                       = HttpRequestSecondsMetric.WithLabelValues("DumpMeta")

	AddPodIntoTenantTotalMetric = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_add_pod_into_tenant_total",
			Help: "The total number of requesting function AddPodIntoTenant",
		},
	)

	AddPodIntoTenantSecondsMetric = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "autoscale_add_pod_into_tenant_seconds",
			Help:    "The duration of requesting function AddPodIntoTenant",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
	)

	AddPodTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_add_pod_total",
			Help: "The total number of added pods",
		},
		[]string{"type"},
	)

	AddPodTotalSuccessMetric = AddPodTotalMetric.WithLabelValues("success")
	AddPodTotalFailedMetric  = AddPodTotalMetric.WithLabelValues("failed")

	RemovePodFromTenantTotalMetric = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_remove_pod_from_tenant_total",
			Help: "The total number of requesting function RemovePodFromTenant",
		},
	)

	RemovePodFromTenantSecondsMetric = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "autoscale_remove_pod_from_tenant_seconds",
			Help:    "The duration of requesting function RemovePodFromTenant",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
	)

	RemovePodTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_remove_pod_total",
			Help: "The total number of removed pods",
		},
		[]string{"type"},
	)

	RemovePodTotalSuccessMetric = RemovePodTotalMetric.WithLabelValues("success")
	RemovePodTotalFailedMetric  = RemovePodTotalMetric.WithLabelValues("failed")

	SupervisorClientRequestTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_request_total",
			Help: "The total number of requests to supervisor",
		},
		[]string{"result"},
	)

	SupervisorClientRequestTotalAssignTenantMetric     = SupervisorClientRequestTotalMetric.WithLabelValues("AssignTenant")
	SupervisorClientRequestTotalUnassignTenantMetric   = SupervisorClientRequestTotalMetric.WithLabelValues("UnassignTenant")
	SupervisorClientRequestTotalGetCurrentTenantMetric = SupervisorClientRequestTotalMetric.WithLabelValues("GetCurrentTenant")

	SupervisorClientRequestSecondsMetric = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_supervisor_client_request_seconds",
			Help:    "The duration of requests to supervisor",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"function"},
	)

	SupervisorClientRequestSecondsAssignTenantMetric     = SupervisorClientRequestSecondsMetric.WithLabelValues("AssignTenant")
	SupervisorClientRequestSecondsUnassignTenantMetric   = SupervisorClientRequestSecondsMetric.WithLabelValues("UnassignTenant")
	SupervisorClientRequestSecondsGetCurrentTenantMetric = SupervisorClientRequestSecondsMetric.WithLabelValues("GetCurrentTenant")

	SupervisorClientUnassignTenantErrorTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_unassign_tenant_error_total",
			Help: "The total number of errors when unassigning tenant"},
		[]string{"type"},
	)

	SupervisorClientUnassignTenantErrorTotalGrpcMetric = SupervisorClientUnassignTenantErrorTotalMetric.WithLabelValues("grpc")
	SupervisorClientUnassignTenantErrorTotalRespMetric = SupervisorClientUnassignTenantErrorTotalMetric.WithLabelValues("response")

	SupervisorClientAssignTenantErrorTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_assign_tenant_error_total",
			Help: "The total number of errors when assigning tenant"},
		[]string{"type"},
	)

	SupervisorClientAssignTenantErrorTotalGrpcMetric = SupervisorClientAssignTenantErrorTotalMetric.WithLabelValues("grpc")
	SupervisorClientAssignTenantErrorTotalRespMetric = SupervisorClientAssignTenantErrorTotalMetric.WithLabelValues("response")

	SupervisorClientGetCurrentTenantErrorTotalMetric = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_get_current_tenant_error_total",
			Help: "The total number of errors when getting current tenant"},
		[]string{"type"},
	)

	SupervisorClientGetCurrentTenantErrorTotalGrpcMetric = SupervisorClientGetCurrentTenantErrorTotalMetric.WithLabelValues("grpc")
	SupervisorClientGetCurrentTenantErrorTotalRespMetric = SupervisorClientGetCurrentTenantErrorTotalMetric.WithLabelValues("response")
)

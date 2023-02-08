package autoscale

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MetricOfTenantSnapshot = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_tenant_count",
		Help: "The current number of tenants",
	})

	MetricOfPodSnapshot = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_pod_count",
		Help: "The current number of pods",
	})

	MetricOfDoPodsWarmSnapshot = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "autoscale_do_pods_warm_count",
			Help: "The current number of pods in function DoPodsWarm",
		}, []string{"type"},
	)

	MetricOfDoPodsWarmFailSnapshot    = MetricOfDoPodsWarmSnapshot.WithLabelValues("fail")
	MetricOfDoPodsWarmDeltaSnapshot   = MetricOfDoPodsWarmSnapshot.WithLabelValues("delta")
	MetricOfDoPodsWarmPendingSnapshot = MetricOfDoPodsWarmSnapshot.WithLabelValues("pending")
	MetricOfDoPodsWarmValidSnapshot   = MetricOfDoPodsWarmSnapshot.WithLabelValues("valid")

	MetricOfWatchPodsLoopEventCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_watch_pods_loop_event_total",
			Help: "The total number of events in function WatchPodsLoop",
		}, []string{"type"},
	)

	MetricOfWatchPodsLoopEventAddedCnt    = MetricOfWatchPodsLoopEventCnt.WithLabelValues("added")
	MetricOfWatchPodsLoopEventModifiedCnt = MetricOfWatchPodsLoopEventCnt.WithLabelValues("modified")
	MetricOfWatchPodsLoopEventDeletedCnt  = MetricOfWatchPodsLoopEventCnt.WithLabelValues("deleted")
	MetricOfWatchPodsLoopEventErrorCnt    = MetricOfWatchPodsLoopEventCnt.WithLabelValues("error")
	MetricOfWatchPodsLoopEventBookmarkCnt = MetricOfWatchPodsLoopEventCnt.WithLabelValues("bookmark")

	MetricOfRpcRequestCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_rpc_request_total",
			Help: "The total number of rpc requests",
		},
		[]string{"type"},
	)

	MetricOfRpcRequestResumeAndGetTopologyCnt = MetricOfRpcRequestCnt.WithLabelValues("ResumeAndGetTopology")
	MetricOfRpcRequestGetTopologyCnt          = MetricOfRpcRequestCnt.WithLabelValues("GetTopology")

	MetricOfRpcRequestSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_rpc_request_seconds",
			Help:    "The duration of rpc requests",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"type"},
	)

	MetricOfRpcRequestResumeAndGetTopologySeconds = MetricOfRpcRequestSeconds.WithLabelValues("ResumeAndGetTopology")
	MetricOfRpcRequestGetTopologySeconds          = MetricOfRpcRequestSeconds.WithLabelValues("GetTopology")

	MetricOfHttpRequestCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_http_request_total",
			Help: "The total number of http requests",
		},
		[]string{"type"},
	)

	MetricOfHttpRequestGetStateServerCnt                 = MetricOfHttpRequestCnt.WithLabelValues("GetStateServer")
	MetricOfHttpRequestSharedFixedPoolCnt                = MetricOfHttpRequestCnt.WithLabelValues("SharedFixedPool")
	MetricOfHttpRequestGetMetricsFromNodeCnt             = MetricOfHttpRequestCnt.WithLabelValues("GetMetricsFromNode")
	MetricOfHttpRequestHttpHandleResumeAndGetTopologyCnt = MetricOfHttpRequestCnt.WithLabelValues("HttpHandleResumeAndGetTopology")
	MetricOfHttpRequestHttpHandlePauseForTestCnt         = MetricOfHttpRequestCnt.WithLabelValues("HttpHandlePauseForTest")
	MetricOfHttpRequestDumpMetaCnt                       = MetricOfHttpRequestCnt.WithLabelValues("DumpMeta")

	MetricOfHttpRequestSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_http_request_seconds",
			Help:    "The duration of http requests",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"type"},
	)

	MetricOfHttpRequestGetStateServerSeconds                       = MetricOfHttpRequestSeconds.WithLabelValues("GetStateServer")
	MetricOfHttpRequestSharedFixedPoolMetricSeconds                = MetricOfHttpRequestSeconds.WithLabelValues("SharedFixedPool")
	MetricOfHttpRequestGetMetricsFromNodeSeconds                   = MetricOfHttpRequestSeconds.WithLabelValues("GetMetricsFromNode")
	MetricOfHttpRequestHttpHandleResumeAndGetTopologyMetricSeconds = MetricOfHttpRequestSeconds.WithLabelValues("HttpHandleResumeAndGetTopology")
	MetricOfHttpRequestHttpHandlePauseForTestMetricSeconds         = MetricOfHttpRequestSeconds.WithLabelValues("HttpHandlePauseForTest")
	MetricOfHttpRequestDumpMetaSeconds                             = MetricOfHttpRequestSeconds.WithLabelValues("DumpMeta")

	MetricOfAddPodIntoTenantRequestCnt = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_add_pod_into_tenant_request_total",
			Help: "The total number of requesting function AddPodIntoTenant",
		},
	)

	MetricOfAddPodIntoTenantSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "autoscale_add_pod_into_tenant_seconds",
			Help:    "The duration of requesting function AddPodIntoTenant",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
	)

	MetricOfAddPodCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_add_pod_total",
			Help: "The total number of added pods",
		},
		[]string{"type"},
	)

	MetricOfAddPodSuccessCnt = MetricOfAddPodCnt.WithLabelValues("success")
	MetricOfAddPodFailedCnt  = MetricOfAddPodCnt.WithLabelValues("failed")

	MetricOfRemovePodFromTenantCnt = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "autoscale_remove_pod_from_tenant_total",
			Help: "The total number of requesting function RemovePodFromTenant",
		},
	)

	MetricOfRemovePodFromTenantSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "autoscale_remove_pod_from_tenant_seconds",
			Help:    "The duration of requesting function RemovePodFromTenant",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
	)

	MetricOfRemovePodCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_remove_pod_total",
			Help: "The total number of removed pods",
		},
		[]string{"type"},
	)

	MetricOfRemovePodSuccessCnt = MetricOfRemovePodCnt.WithLabelValues("success")
	MetricOfRemovePodFailedCnt  = MetricOfRemovePodCnt.WithLabelValues("failed")

	MetricOfSupervisorClientRequestCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_request_total",
			Help: "The total number of requests to supervisor",
		},
		[]string{"type"},
	)

	MetricOfSupervisorClientRequestAssignTenantCnt     = MetricOfSupervisorClientRequestCnt.WithLabelValues("AssignTenant")
	MetricOfSupervisorClientRequestUnassignTenantCnt   = MetricOfSupervisorClientRequestCnt.WithLabelValues("UnassignTenant")
	MetricOfSupervisorClientRequestGetCurrentTenantCnt = MetricOfSupervisorClientRequestCnt.WithLabelValues("GetCurrentTenant")

	MetricOfSupervisorClientRequestSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_supervisor_client_request_seconds",
			Help:    "The duration of requests to supervisor",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"type"},
	)

	MetricOfSupervisorClientRequestAssignTenantSeconds     = MetricOfSupervisorClientRequestSeconds.WithLabelValues("AssignTenant")
	MetricOfSupervisorClientRequestUnassignTenantSeconds   = MetricOfSupervisorClientRequestSeconds.WithLabelValues("UnassignTenant")
	MetricOfSupervisorClientRequestGetCurrentTenantSeconds = MetricOfSupervisorClientRequestSeconds.WithLabelValues("GetCurrentTenant")

	MetricOfSupervisorClientUnassignTenantErrorCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_unassign_tenant_error_total",
			Help: "The total number of errors when unassigning tenant"},
		[]string{"type"},
	)

	MetricOfSupervisorClientUnassignTenantErrorGrpcCnt = MetricOfSupervisorClientUnassignTenantErrorCnt.WithLabelValues("grpc")
	MetricOfSupervisorClientUnassignTenantErrorRespCnt = MetricOfSupervisorClientUnassignTenantErrorCnt.WithLabelValues("response")

	MetricOfSupervisorClientAssignTenantErrorCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_assign_tenant_error_total",
			Help: "The total number of errors when assigning tenant"},
		[]string{"type"},
	)

	MetricOfSupervisorClientAssignTenantErrorGrpcCnt = MetricOfSupervisorClientAssignTenantErrorCnt.WithLabelValues("grpc")
	MetricOfSupervisorClientAssignTenantErrorRespCnt = MetricOfSupervisorClientAssignTenantErrorCnt.WithLabelValues("response")

	MetricOfSupervisorClientGetCurrentTenantErrorCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_get_current_tenant_error_total",
			Help: "The total number of errors when getting current tenant"},
		[]string{"type"},
	)

	MetricOfSupervisorClientGetCurrentTenantErrorGrpcCnt = MetricOfSupervisorClientGetCurrentTenantErrorCnt.WithLabelValues("grpc")
	//MetricOfSupervisorClientGetCurrentTenantErrorRespCnt = MetricOfSupervisorClientGetCurrentTenantErrorCnt.WithLabelValues("response")
)

package autoscale

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MetricOfTenantCntSnapshot = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_tenant_count_snapshot",
		Help: "The snapshot of tenant count",
	})

	MetricOfPodCntSnapshot = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "autoscale_pod_count_snapshot",
		Help: "The snapshot of pod count",
	})

	MetricOfDoPodsWarmSnapshot = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "autoscale_do_pods_warm_snapshot",
			Help: "The snapshot in function DoPodsWarm",
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

	MetricOfRpcRequestResumeAndGetTopologyCnt = MetricOfRpcRequestCnt.WithLabelValues("resume_and_get_topology")
	MetricOfRpcRequestGetTopologyCnt          = MetricOfRpcRequestCnt.WithLabelValues("get_topology")

	MetricOfRpcRequestSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_rpc_request_duration_seconds",
			Help:    "The duration of rpc requests",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"type"},
	)

	MetricOfRpcRequestResumeAndGetTopologySeconds = MetricOfRpcRequestSeconds.WithLabelValues("resume_and_get_topology")
	MetricOfRpcRequestGetTopologySeconds          = MetricOfRpcRequestSeconds.WithLabelValues("get_topology")

	MetricOfHttpRequestCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_http_request_total",
			Help: "The total number of http requests",
		},
		[]string{"type"},
	)

	MetricOfHttpRequestGetStateServerCnt                 = MetricOfHttpRequestCnt.WithLabelValues("get_state_server")
	MetricOfHttpRequestSharedFixedPoolCnt                = MetricOfHttpRequestCnt.WithLabelValues("shared_fixed_pool")
	MetricOfHttpRequestGetMetricsFromNodeCnt             = MetricOfHttpRequestCnt.WithLabelValues("get_metrics_from_node")
	MetricOfHttpRequestHttpHandleResumeAndGetTopologyCnt = MetricOfHttpRequestCnt.WithLabelValues("http_handle_resume_and_get_topology")
	MetricOfHttpRequestHttpHandlePauseForTestCnt         = MetricOfHttpRequestCnt.WithLabelValues("http_handle_pause_for_test")
	MetricOfHttpRequestDumpMetaCnt                       = MetricOfHttpRequestCnt.WithLabelValues("dump_meta")

	MetricOfHttpRequestSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_http_request_duration_seconds",
			Help:    "The duration of http requests",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"type"},
	)

	MetricOfHttpRequestGetStateServerSeconds                       = MetricOfHttpRequestSeconds.WithLabelValues("get_state_server")
	MetricOfHttpRequestSharedFixedPoolMetricSeconds                = MetricOfHttpRequestSeconds.WithLabelValues("shared_fixed_pool")
	MetricOfHttpRequestGetMetricsFromNodeSeconds                   = MetricOfHttpRequestSeconds.WithLabelValues("get_metrics_from_node")
	MetricOfHttpRequestHttpHandleResumeAndGetTopologyMetricSeconds = MetricOfHttpRequestSeconds.WithLabelValues("http_handle_resume_and_get_topology")
	MetricOfHttpRequestHttpHandlePauseForTestMetricSeconds         = MetricOfHttpRequestSeconds.WithLabelValues("http_handle_pause_for_test")
	MetricOfHttpRequestDumpMetaSeconds                             = MetricOfHttpRequestSeconds.WithLabelValues("dump_meta")

	MetricOfChangeOfPodOnTenantCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_changes_of_pod_on_tenant_total",
			Help: "The total number of pods changed on tenant",
		},
		[]string{"type"},
	)

	MetricOfChangeOfPodOnTenantSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_changes_of_pod_on_tenant_duration_seconds",
			Help:    "The duration of changing pods on tenant",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"type"},
	)

	MetricOfAddPodIntoTenantCnt        = MetricOfChangeOfPodOnTenantCnt.WithLabelValues("add")
	MetricOfRemovePodFromTenantCnt     = MetricOfChangeOfPodOnTenantCnt.WithLabelValues("remove")
	MetricOfAddPodIntoTenantSeconds    = MetricOfChangeOfPodOnTenantSeconds.WithLabelValues("add")
	MetricOfRemovePodFromTenantSeconds = MetricOfChangeOfPodOnTenantSeconds.WithLabelValues("remove")

	MetricOfChangeOfPodCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_changes_of_pod_total",
			Help: "The total number of changed pods",
		},
		[]string{"type"},
	)

	MetricOfAddPodSuccessCnt    = MetricOfChangeOfPodCnt.WithLabelValues("add")
	MetricOfAddPodFailedCnt     = MetricOfChangeOfPodCnt.WithLabelValues("add_failed")
	MetricOfRemovePodSuccessCnt = MetricOfChangeOfPodCnt.WithLabelValues("remove")
	MetricOfRemovePodFailedCnt  = MetricOfChangeOfPodCnt.WithLabelValues("remove_failed")

	MetricOfSupervisorClientRequestCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_supervisor_client_request_total",
			Help: "The total number of requests to supervisor",
		},
		[]string{"type"},
	)

	MetricOfSupervisorClientRequestAssignTenantCnt     = MetricOfSupervisorClientRequestCnt.WithLabelValues("assign_tenant")
	MetricOfSupervisorClientRequestUnassignTenantCnt   = MetricOfSupervisorClientRequestCnt.WithLabelValues("unassign_tenant")
	MetricOfSupervisorClientRequestGetCurrentTenantCnt = MetricOfSupervisorClientRequestCnt.WithLabelValues("get_current_tenant")

	MetricOfSupervisorClientAssignTenantErrorGrpcCnt     = MetricOfSupervisorClientRequestCnt.WithLabelValues("assign_tenant_grpc_fail")
	MetricOfSupervisorClientAssignTenantErrorRespCnt     = MetricOfSupervisorClientRequestCnt.WithLabelValues("assign_tenant_response_fail")
	MetricOfSupervisorClientUnassignTenantErrorGrpcCnt   = MetricOfSupervisorClientRequestCnt.WithLabelValues("unassign_tenant_grpc_fail")
	MetricOfSupervisorClientUnassignTenantErrorRespCnt   = MetricOfSupervisorClientRequestCnt.WithLabelValues("unassign_tenant_response_fail")
	MetricOfSupervisorClientGetCurrentTenantErrorGrpcCnt = MetricOfSupervisorClientRequestCnt.WithLabelValues("get_current_tenant_grpc_fail")

	MetricOfSupervisorClientRequestSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "autoscale_supervisor_client_request_duration_seconds",
			Help:    "The duration of requests to supervisor",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 10),
		},
		[]string{"type"},
	)

	MetricOfSupervisorClientRequestAssignTenantSeconds     = MetricOfSupervisorClientRequestSeconds.WithLabelValues("assign_tenant")
	MetricOfSupervisorClientRequestUnassignTenantSeconds   = MetricOfSupervisorClientRequestSeconds.WithLabelValues("unassign_tenant")
	MetricOfSupervisorClientRequestGetCurrentTenantSeconds = MetricOfSupervisorClientRequestSeconds.WithLabelValues("get_current_tenant")

	MetricOfChangeOfClonesetReplicaCnt = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "autoscale_cloneset_replica_change_total",
			Help: "The total number of replicas changed in cloneset",
		},
		[]string{"type"},
	)

	MetricOfClonesetReplicaAddSuccessCnt = MetricOfChangeOfClonesetReplicaCnt.WithLabelValues("add")
	MetricOfClonesetReplicaAddFailedCnt  = MetricOfChangeOfClonesetReplicaCnt.WithLabelValues("add_failed")

	MetricOfClonesetReplicaDelSuccessCnt = MetricOfChangeOfClonesetReplicaCnt.WithLabelValues("delete")
	MetricOfClonesetReplicaDelFailedCnt  = MetricOfChangeOfClonesetReplicaCnt.WithLabelValues("delete_failed")
)

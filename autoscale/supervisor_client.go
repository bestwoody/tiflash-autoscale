package autoscale

import (
	"context"
	"fmt"
	"time"

	supervisor "github.com/tikv/pd/supervisor_proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	SupervisorPort         string = "7000"
	IsSupClientMock        bool   = false
	GrpcCommonTimeOutSec          = 10
	GrpcAssignTimeOutSec          = 120
	GrpcUnassignTimeOutSec        = 120
)

var HardCodeEnvTidbStatusAddr string
var HardCodeEnvPdAddr string
var HardCodeSupervisorImage string

var (
	SupervisorClientRequestTotalAssignTenant     = SupervisorClientRequestTotal.WithLabelValues("AssignTenant")
	SupervisorClientRequestTotalUnassignTenant   = SupervisorClientRequestTotal.WithLabelValues("UnassignTenant")
	SupervisorClientRequestTotalGetCurrentTenant = SupervisorClientRequestTotal.WithLabelValues("GetCurrentTenant")

	SupervisorClientRequestMSAssignTenant     = SupervisorClientRequestMS.WithLabelValues("AssignTenant")
	SupervisorClientRequestMSUnassignTenant   = SupervisorClientRequestMS.WithLabelValues("UnassignTenant")
	SupervisorClientRequestMSGetCurrentTenant = SupervisorClientRequestMS.WithLabelValues("GetCurrentTenant")
)

func AssignTenantHardCodeArgs(podIP string, tenantName string) (resp *supervisor.Result, err error) {
	return AssignTenant(podIP, tenantName, HardCodeEnvTidbStatusAddr, HardCodeEnvPdAddr)
}

func AssignTenant(podIP string, tenantName string, tidbStatusAddr string, pdAddr string) (resp *supervisor.Result, err error) {
	start := time.Now()
	SupervisorClientRequestTotalAssignTenant.Inc()
	defer func() {
		if err != nil || (resp != nil && resp.HasErr) {
			var err1, err2 string
			if err != nil {
				err1 = err.Error()
			}
			if resp != nil && resp.HasErr {
				err2 = resp.ErrInfo
			}
			Logger.Errorf("[error][SupClient]failed to AssignTenant, grpc_err: %v  api_err: %v", err1, err2)
		}
		SupervisorClientRequestMSAssignTenant.Observe(float64(time.Since(start).Milliseconds()))
	}()
	if !IsSupClientMock {
		Logger.Infof("[SupClient][AssignTenant]grpc dial addr: %v ", podIP+":"+SupervisorPort)
		connctx, conncancel := context.WithTimeout(context.Background(), GrpcCommonTimeOutSec*time.Second)
		defer conncancel()
		conn, err := grpc.DialContext(connctx, podIP+":"+SupervisorPort, grpc.WithInsecure(), grpc.FailOnNonTempDialError(true), grpc.WithBlock())
		if err != nil {
			return nil, err
		}
		if conn != nil {
			defer conn.Close()
		}
		c := supervisor.NewAssignClient(conn)

		// Contact the server and print out its response.
		ctx, cancel := context.WithTimeout(context.Background(), GrpcAssignTimeOutSec*time.Second)
		defer cancel()
		r, err :=
			c.AssignTenant(
				ctx,
				&supervisor.AssignRequest{TenantID: tenantName, TidbStatusAddr: tidbStatusAddr, PdAddr: pdAddr})
		if err != nil {
			Logger.Errorf("[error][SupClient]AssignTenant fail: %v , podIP: %v ", err, podIP)
			return r, err
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			Logger.Infof("[SupClient][AssignTenant]result: %v , podIP: %v ", respStr, podIP)
		}
		// Logger.Infof("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return &supervisor.Result{HasErr: false}, nil
	}
}

func UnassignTenant(podIP string, tenantName string, forceShutdown bool) (resp *supervisor.Result, err error) {
	start := time.Now()
	SupervisorClientRequestTotalUnassignTenant.Inc()
	defer func() {
		if err != nil || (resp != nil && resp.HasErr) {
			var err1, err2 string
			if err != nil {
				err1 = err.Error()
			}
			if resp != nil && resp.HasErr {
				err2 = resp.ErrInfo
			}
			Logger.Errorf("[error][SupClient]failed to UnassignTenant, grpc_err: %v  api_err: %v", err1, err2)
		}
		SupervisorClientRequestMSUnassignTenant.Observe(float64(time.Since(start).Milliseconds()))
	}()
	if !IsSupClientMock {
		Logger.Infof("[SupClient][UnassignTenant]grpc dial addr: %v ", podIP+":"+SupervisorPort)
		connctx, conncancel := context.WithTimeout(context.Background(), GrpcCommonTimeOutSec*time.Second)
		defer conncancel()
		conn, err := grpc.DialContext(connctx, podIP+":"+SupervisorPort, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			return nil, err
		}
		if conn != nil {
			defer conn.Close()
		}
		c := supervisor.NewAssignClient(conn)

		// Contact the server and print out its response.
		ctx, cancel := context.WithTimeout(context.Background(), GrpcUnassignTimeOutSec*time.Second)
		defer cancel()
		r, err := c.UnassignTenant(ctx, &supervisor.UnassignRequest{AssertTenantID: tenantName, ForceShutdown: forceShutdown})
		if err != nil {
			Logger.Errorf("[error][SupClient]UnassignTenant fail: %v, podIP:%v ", err, podIP)
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			Logger.Infof("[SupClient][UnAssignTenant]result: %v , podIP: %v ", respStr, podIP)
		}
		// Logger.Infof("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return &supervisor.Result{HasErr: false}, nil
	}
}

func GetCurrentTenant(podIP string) (resp *supervisor.GetTenantResponse, err error) {
	start := time.Now()
	SupervisorClientRequestTotalGetCurrentTenant.Inc()
	defer func() {
		if err != nil {
			Logger.Errorf("[error][SupClient]failed to GetCurrentTenant, grpc_err: %v", err.Error())
		}
		SupervisorClientRequestMSGetCurrentTenant.Observe(float64(time.Since(start).Milliseconds()))
	}()
	if !IsSupClientMock {
		Logger.Infof("[GetCurrentTenant]grpc dial addr: %v ", podIP+":"+SupervisorPort)
		ctx, cancel := context.WithTimeout(context.Background(), GrpcCommonTimeOutSec*time.Second)
		defer cancel()
		conn, err := grpc.DialContext(ctx, podIP+":"+SupervisorPort, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			return nil, err
		}
		if conn != nil {
			defer conn.Close()
		}
		c := supervisor.NewAssignClient(conn)

		// Contact the server and print out its response.
		ctx2, cancel2 := context.WithTimeout(context.Background(), GrpcCommonTimeOutSec*time.Second)
		defer cancel2()
		r, err := c.GetCurrentTenant(ctx2, &emptypb.Empty{})
		if err != nil {
			Logger.Errorf("[error][SupClient]GetCurrentTenant fail: %v, podIp: %v", err, podIP)
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			Logger.Infof("[SupClient][GetTenant]result: %v , podIP: %v ", respStr, podIP)
		}
		// Logger.Infof("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return nil, fmt.Errorf("mock GetCurrentTenant")
	}
}

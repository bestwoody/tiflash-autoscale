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
	SupervisorPort  string = "7000"
	IsSupClientMock bool   = false
)

var HardCodeEnvTidbStatusAddr string
var HardCodeEnvPdAddr string

func AssignTenantHardCodeArgs(podIP string, tenantName string) (resp *supervisor.Result, err error) {
	return AssignTenant(podIP, tenantName, HardCodeEnvTidbStatusAddr, HardCodeEnvPdAddr)
}

func AssignTenant(podIP string, tenantName string, tidbStatusAddr string, pdAddr string) (resp *supervisor.Result, err error) {
	defer func() {
		if err != nil || (resp != nil && resp.HasErr) {
			var err1, err2 string
			if err != nil {
				err1 = err.Error()
			}
			if resp != nil && resp.HasErr {
				err2 = resp.ErrInfo
			}
			Logger.Errorf("[error]failed to AssignTenant, grpc_err: %v  api_err: %v", err1, err2)
		}
	}()
	if !IsSupClientMock {
		Logger.Infof("[AssignTenant]grpc dial addr: %v ", podIP+":"+SupervisorPort)
		conn, err := grpc.Dial(podIP+":"+SupervisorPort, grpc.WithInsecure(), grpc.FailOnNonTempDialError(true), grpc.WithBlock())
		if err != nil {
			return nil, err
		}
		if conn != nil {
			defer conn.Close()
		}
		c := supervisor.NewAssignClient(conn)

		// Contact the server and print out its response.
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()
		r, err :=
			c.AssignTenant(
				ctx,
				&supervisor.AssignRequest{TenantID: tenantName, TidbStatusAddr: tidbStatusAddr, PdAddr: pdAddr})
		if err != nil {
			Logger.Errorf("[error]AssignTenant fail: %v , podIP: %v ", err, podIP)
			return r, err
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			Logger.Infof("[AssignTenant]result: %v , podIP: %v ", respStr, podIP)
		}
		// Logger.Infof("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return &supervisor.Result{HasErr: false}, nil
	}
}

func UnassignTenant(podIP string, tenantName string) (resp *supervisor.Result, err error) {
	defer func() {
		if err != nil || (resp != nil && resp.HasErr) {
			var err1, err2 string
			if err != nil {
				err1 = err.Error()
			}
			if resp != nil && resp.HasErr {
				err2 = resp.ErrInfo
			}
			Logger.Errorf("[error]failed to UnassignTenant, grpc_err: %v  api_err: %v", err1, err2)
		}
	}()
	if !IsSupClientMock {
		Logger.Infof("[UnassignTenant]grpc dial addr: %v ", podIP+":"+SupervisorPort)
		conn, err := grpc.Dial(podIP+":"+SupervisorPort, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			return nil, err
		}
		if conn != nil {
			defer conn.Close()
		}
		c := supervisor.NewAssignClient(conn)

		// Contact the server and print out its response.
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()
		r, err := c.UnassignTenant(ctx, &supervisor.UnassignRequest{AssertTenantID: tenantName})
		if err != nil {
			Logger.Errorf("[error]UnassignTenant fail: %v, podIP:%v ", err, podIP)
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			Logger.Infof("[UnAssignTenant]result: %v , podIP: %v ", respStr, podIP)
		}
		// Logger.Infof("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return &supervisor.Result{HasErr: false}, nil
	}
}

func GetCurrentTenant(podIP string) (resp *supervisor.GetTenantResponse, err error) {
	defer func() {
		if err != nil {
			Logger.Errorf("[error]failed to GetCurrentTenant, grpc_err: %v", err.Error())
		}
	}()
	if !IsSupClientMock {
		Logger.Infof("[GetCurrentTenant]grpc dial addr: %v ", podIP+":"+SupervisorPort)
		conn, err := grpc.Dial(podIP+":"+SupervisorPort, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			return nil, err
		}
		if conn != nil {
			defer conn.Close()
		}
		c := supervisor.NewAssignClient(conn)

		// Contact the server and print out its response.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r, err := c.GetCurrentTenant(ctx, &emptypb.Empty{})
		if err != nil {
			Logger.Errorf("[error]GetCurrentTenant fail: %v, podIp: %v", err, podIP)
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			Logger.Infof("[GetTenant]result: %v , podIP: %v ", respStr, podIP)
		}
		// Logger.Infof("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return nil, fmt.Errorf("mock GetCurrentTenant")
	}
}

package autoscale

import (
	"context"
	"fmt"
	"log"
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

func AssignTenant(podIP string, tenantName string) (resp *supervisor.Result, err error) {
	defer func() {
		if err != nil || (resp != nil && resp.HasErr) {
			var err1, err2 string
			if err != nil {
				err1 = err.Error()
			}
			if resp != nil && resp.HasErr {
				err2 = resp.ErrInfo
			}
			log.Printf("[error]failed to AssignTenant, grpc_err: %v  api_err: %v\n", err1, err2)
		}
	}()
	if !IsSupClientMock {
		log.Printf("[AssignTenant]grpc dial addr: %v \n", podIP+":"+SupervisorPort)
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
		r, err := c.AssignTenant(ctx, &supervisor.AssignRequest{TenantID: tenantName, TidbStatusAddr: HardCodeEnvTidbStatusAddr, PdAddr: HardCodeEnvPdAddr})
		if err != nil {
			log.Printf("[error]AssignTenant fail: %v , podIP: %v \n", err, podIP)
			return r, err
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			log.Printf("[AssignTenant]result: %v , podIP: %v \n", respStr, podIP)
		}
		// log.Printf("result: %s", r.HasErr)
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
			log.Printf("[error]failed to UnassignTenant, grpc_err: %v  api_err: %v\n", err1, err2)
		}
	}()
	if !IsSupClientMock {
		log.Printf("[UnassignTenant]grpc dial addr: %v \n", podIP+":"+SupervisorPort)
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
			log.Printf("[error]UnassignTenant fail: %v, podIP:%v \n", err, podIP)
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			log.Printf("[UnAssignTenant]result: %v , podIP: %v \n", respStr, podIP)
		}
		// log.Printf("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return &supervisor.Result{HasErr: false}, nil
	}
}

func GetCurrentTenant(podIP string) (resp *supervisor.GetTenantResponse, err error) {
	defer func() {
		if err != nil {
			log.Printf("[error]failed to GetCurrentTenant, grpc_err: %v\n", err.Error())
		}
	}()
	if !IsSupClientMock {
		log.Printf("[GetCurrentTenant]grpc dial addr: %v \n", podIP+":"+SupervisorPort)
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
			log.Printf("[error]GetCurrentTenant fail: %v, podIp: %v\n", err, podIP)
		} else {
			respStr := r.TenantID
			if r.TenantID == "" {
				respStr = "empty"
			}
			log.Printf("[GetTenant]result: %v , podIP: %v \n", respStr, podIP)
		}
		// log.Printf("result: %s", r.HasErr)
		return r, err
	} else {
		time.Sleep(500 * time.Millisecond)
		return nil, fmt.Errorf("mock GetCurrentTenant")
	}
}

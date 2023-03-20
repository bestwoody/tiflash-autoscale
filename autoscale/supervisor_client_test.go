package autoscale

import (
	"context"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	pb "github.com/tikv/pd/supervisor_proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type FakeAssignServer struct {
	// mock variables in server
	tenantName    atomic.Value
	isTimeoutCase bool
}

func (s *FakeAssignServer) AssignTenant(ctx context.Context, in *pb.AssignRequest) (*pb.Result, error) {
	s.tenantName.Store(in.GetTenantID())
	return &pb.Result{HasErr: false, ErrInfo: "", TenantID: s.tenantName.Load().(string), IsUnassigning: false}, nil
}

func (s *FakeAssignServer) UnassignTenant(ctx context.Context, in *pb.UnassignRequest) (*pb.Result, error) {
	if in.AssertTenantID != s.tenantName.Load() {
		return &pb.Result{HasErr: true, ErrInfo: "TiFlash is not assigned to this tenant", TenantID: s.tenantName.Load().(string), IsUnassigning: false}, nil
	}
	s.tenantName.Store("")
	return &pb.Result{HasErr: false, ErrInfo: "", TenantID: s.tenantName.Load().(string), IsUnassigning: false}, nil
}

func (s *FakeAssignServer) GetCurrentTenant(ctx context.Context, empty *emptypb.Empty) (*pb.GetTenantResponse, error) {
	if s.isTimeoutCase {
		time.Sleep((GrpcCommonTimeOutSec + 5) * time.Second)
	}
	return &pb.GetTenantResponse{TenantID: s.tenantName.Load().(string), IsUnassigning: false}, nil
}

// If no error is returned, the close function will not be nil.
func InitRpcTestEnv(fakeServer *FakeAssignServer) (func(), error) {
	lis, err := net.Listen("tcp", ":"+SupervisorPort)
	if err != nil {
		return nil, err
	}
	s := grpc.NewServer()
	pb.RegisterAssignServer(s, fakeServer)
	log.Printf("server listening at %v", lis.Addr())
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	closer := func() {
		s.Stop()
		log.Printf("grpc server closes")
		return
	}
	return closer, nil
}

func TestAssignAndUnassignTenant(t *testing.T) {
	IsSupClientMock = false
	podIP := "127.0.0.1"
	tenantName := "test-tenant"
	tenantName2 := "test-tenant2"
	pdAddr := "123.123.123.123"
	InitTestEnv()
	fakeAssignServer := &FakeAssignServer{}
	closer, err := InitRpcTestEnv(fakeAssignServer)
	assert.NoError(t, err)
	defer closer()

	assignTenantResult, err := AssignTenantHardCodeArgs(podIP, tenantName, pdAddr, "")
	assert.NoError(t, err)
	assert.False(t, assignTenantResult.HasErr)
	assertEqual(t, assignTenantResult.TenantID, tenantName)
	assert.False(t, assignTenantResult.IsUnassigning)

	getCurrentTenantResult, err := GetCurrentTenant(podIP)
	assert.NoError(t, err)
	assert.False(t, getCurrentTenantResult.IsUnassigning)
	assert.Equal(t, getCurrentTenantResult.TenantID, tenantName)

	unassignTenantResult, err := UnassignTenant(podIP, tenantName2, true)
	assert.NoError(t, err)
	assert.True(t, unassignTenantResult.HasErr)
	assert.True(t, strings.Contains(unassignTenantResult.ErrInfo, "TiFlash is not assigned to this tenant"))
	assert.Equal(t, unassignTenantResult.TenantID, tenantName)
	assert.False(t, unassignTenantResult.IsUnassigning)

	unassignTenantResult, err = UnassignTenant(podIP, tenantName, true)
	assert.NoError(t, err)
	assert.False(t, unassignTenantResult.HasErr)
	assert.Equal(t, unassignTenantResult.TenantID, "")
	assert.False(t, unassignTenantResult.IsUnassigning)

	unassignTenantResult, err = UnassignTenant(podIP, tenantName, true)
	assert.NoError(t, err)
	assert.True(t, unassignTenantResult.HasErr)
	assert.True(t, strings.Contains(unassignTenantResult.ErrInfo, "TiFlash is not assigned to this tenant"))
	assert.Equal(t, unassignTenantResult.TenantID, "")
	assert.False(t, unassignTenantResult.IsUnassigning)

	// test timeout case
	fakeAssignServer.isTimeoutCase = true
	_, err = GetCurrentTenant(podIP)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "context deadline exceeded"))

}

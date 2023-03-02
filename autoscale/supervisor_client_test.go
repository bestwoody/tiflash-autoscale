package autoscale

import (
	context "context"
	pb "github.com/tikv/pd/supervisor_proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"sync/atomic"
	"testing"
)

var (
	podIP      = "127.0.0.1"
	tenantName = "test-tenant"

	// mock variables in server
	tenantNameInServer atomic.Value
	isUnassigning      atomic.Bool
)

func (s *server) AssignTenant(ctx context.Context, in *pb.AssignRequest) (*pb.Result, error) {
	tenantNameInServer.Store(in.GetTenantID())
	return &pb.Result{HasErr: false, ErrInfo: "", TenantID: tenantNameInServer.Load().(string), IsUnassigning: isUnassigning.Load()}, nil
}

func (s *server) UnassignTenant(ctx context.Context, in *pb.UnassignRequest) (*pb.Result, error) {
	tenantNameInServer.Store("")
	if in.AssertTenantID != tenantNameInServer.Load() {
		return &pb.Result{HasErr: true, ErrInfo: "assert tenant id failed", TenantID: tenantNameInServer.Load().(string), IsUnassigning: isUnassigning.Load()}, nil
	}
	return &pb.Result{HasErr: false, ErrInfo: "", TenantID: tenantNameInServer.Load().(string), IsUnassigning: isUnassigning.Load()}, nil
}

func (s *server) GetCurrentTenant(ctx context.Context, empty *emptypb.Empty) (*pb.GetTenantResponse, error) {
	return &pb.GetTenantResponse{TenantID: tenantNameInServer.Load().(string), IsUnassigning: isUnassigning.Load()}, nil
}

func TestAssignAndUnassignTenant(t *testing.T) {
	AssignTenantHardCodeArgs(podIP, tenantName)
	UnassignTenant(podIP, tenantName, true)
}

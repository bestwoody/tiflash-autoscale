/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package autoscale

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/tikv/pd/auto_scale_proto"
	"google.golang.org/grpc"
)

type server struct {
	// pb.UnimplementedAutoScaleServer
}

func (s *server) GetTopology(ctx context.Context, in *pb.GetTopologyRequest) (*pb.GetTopologyResponse, error) {
	now := time.Now()
	ts := now.UnixNano()

	topoList := GetTopology(in.GetTidbClusterID())
	return &pb.GetTopologyResponse{TidbClusterID: in.TidbClusterID, Timestamp: ts, TopologyList: topoList}, nil
}

func (s *server) ResumeAndGetTopology(ctx context.Context, req *pb.ResumeAndGetTopologyRequest) (*pb.ResumeAndGetTopologyResponse, error) {
	st := time.Now()
	ret := &pb.ResumeAndGetTopologyResponse{}
	flag := Cm4Http.Resume(req.GetTidbClusterID())
	if !flag {
		ret.HasErr = true
		ret.ErrInfo = ("resume failed")
	}
	flag, curState, _ := Cm4Http.AutoScaleMeta.GetTenantState(req.GetTidbClusterID())
	if !flag {
		ret.HasErr = true
		ret.ErrInfo = ("no such tidb-cluster")
		return ret, nil
	} else {
		ret.CurState = ConvertStateString(curState)
	}

	TimeOutSec := int64(60)
	for time.Now().Unix()-st.Unix() <= TimeOutSec {
		topoList := GetTopology(req.GetTidbClusterID())

		if len(topoList) == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			ret.Topology = &pb.GetTopologyResponse{TidbClusterID: req.GetTidbClusterID(), Timestamp: time.Now().UnixNano(), TopologyList: topoList}
			return ret, nil
		}
	}
	ret.HasErr = true
	ret.ErrInfo = "timeout"
	return ret, nil
}

func GetTopology(tidbClusterID string) []string {
	return Cm4Http.AutoScaleMeta.GetTopology(tidbClusterID)
}

func RunGrpcServer() {
	listener, err := net.Listen("tcp", ":8091")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterAutoScaleServer(s, &server{})
	Logger.Infof("[grpc] server listening at %v", listener.Addr())
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

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
	pb "github.com/tikv/pd/auto_scale_proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedAutoScaleServer
}

func (s *server) GetTopo(ctx context.Context, in *pb.GetTopologyRequest) (*pb.GetTopologyResponse, error) {
	ts := timestamppb.Now()

	topoList := GetTopology(in.GetTidbClusterID())
	return &pb.GetTopologyResponse{TidbClusterID: in.TidbClusterID, Timestamp: ts, TopologyList: topoList}, nil
}

func GetTopology(tidbClusterID string) []string {
	return []string{"aaa,bbb"}
}

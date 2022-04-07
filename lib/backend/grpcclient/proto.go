// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpcclient

import (
	"github.com/gravitational/teleport/lib/backend"

	bpb "github.com/gravitational/teleport/api/backend/proto"
)

func grpcItemsToBackendItems(in []*bpb.Item) []backend.Item {
	result := make([]backend.Item, len(in))
	for i, v := range in {
		result[i] = *grpcItemToBackendItem(v)
	}
	return result
}

func grpcItemToBackendItem(in *bpb.Item) *backend.Item {
	if in == nil {
		return &backend.Item{}
	}

	return &backend.Item{
		Key:     in.Key,
		Value:   in.Value,
		Expires: in.Expires,
		ID:      in.Id,
		LeaseID: in.LeaseId,
	}
}

func grpcLeaseToBackendLease(in *bpb.Lease) *backend.Lease {
	if in == nil {
		return &backend.Lease{}
	}
	return &backend.Lease{
		ID:  in.Id,
		Key: in.Key,
	}
}

func backendItemToGrpcItem(in backend.Item) *bpb.Item {
	return &bpb.Item{
		Key:     in.Key,
		Value:   in.Value,
		Expires: in.Expires,
		Id:      in.ID,
		LeaseId: in.LeaseID,
	}
}

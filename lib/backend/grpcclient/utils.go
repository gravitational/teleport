/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package grpcclient

import (
	grpcbackend "github.com/gravitational/teleport/api/backend/grpc"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

func ConvertGrpcEventBackendEvent(in *grpcbackend.Event) backend.Event {
	return backend.Event{
		Item: *ConvertGrpcItemBackendItem(in.Item),
		Type: toBackendOpType(in.Type),
	}
}

func ConvertBackendEventGrpcEvent(in *backend.Event) *grpcbackend.Event {
	return &grpcbackend.Event{
		Item: ConvertBackendItemGrpcItem(in.Item),
		Type: toGrpcOpType(in.Type),
	}
}

func ConvertGrpcItemBackendItem(in *grpcbackend.Item) *backend.Item {
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

func ConvertBackendItemGrpcItem(in backend.Item) *grpcbackend.Item {
	return &grpcbackend.Item{
		Key:     in.Key,
		Value:   in.Value,
		Expires: in.Expires,
		Id:      in.ID,
		LeaseId: in.LeaseID,
	}
}

func toBackendOpType(in grpcbackend.OpType) types.OpType {
	// TODO(knisbet): normally I would replace the internal
	// representation with grpc instead of doing these types
	// of conversions. But just convert while experimental
	switch in {
	case grpcbackend.OpType_op_delete:
		return types.OpDelete
	case grpcbackend.OpType_op_get:
		return types.OpGet
	case grpcbackend.OpType_op_init:
		return types.OpInit
	case grpcbackend.OpType_op_invalid:
		return types.OpInvalid
	case grpcbackend.OpType_op_put:
		return types.OpPut
	case grpcbackend.OpType_op_unreliable:
		return types.OpUnreliable
	default:
		panic("mismatched enum conversion to internal type")
	}
}

func toGrpcOpType(in types.OpType) grpcbackend.OpType {
	// TODO(knisbet): normally I would replace the internal
	// representation with grpc instead of doing these types
	// of conversions. But just convert while experimental
	switch in {
	case types.OpDelete:
		return grpcbackend.OpType_op_delete
	case types.OpGet:
		return grpcbackend.OpType_op_get
	case types.OpInit:
		return grpcbackend.OpType_op_init
	case types.OpInvalid:
		return grpcbackend.OpType_op_invalid
	case types.OpPut:
		return grpcbackend.OpType_op_put
	case types.OpUnreliable:
		return grpcbackend.OpType_op_unreliable
	default:
		panic("mismatched enum conversion to internal type")
	}
}

func ConvertGrpcItemsBackendItems(in []*grpcbackend.Item) []backend.Item {
	result := make([]backend.Item, 0, len(in))

	for i, v := range in {
		result[i] = *ConvertGrpcItemBackendItem(v)
	}

	return result
}

func ConvertBackendItemsGrpcItems(in []backend.Item) []*grpcbackend.Item {
	if in == nil {
		return nil
	}
	result := make([]*grpcbackend.Item, 0, len(in))

	for i, v := range in {
		result[i] = ConvertBackendItemGrpcItem(v)
	}

	return result
}

func ConvertGrpcLeaseBackendLease(in *grpcbackend.Lease) *backend.Lease {
	if in == nil {
		return &backend.Lease{}
	}
	return &backend.Lease{
		ID:  in.Id,
		Key: in.Key,
	}
}

func ConvertBackendLeaseGrpcLease(in backend.Lease) *grpcbackend.Lease {
	return &grpcbackend.Lease{
		Id:  in.ID,
		Key: in.Key,
	}
}

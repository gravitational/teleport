/*
Copyright 2020 Gravitational, Inc.

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
package auth

import (
	"context"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc"
)

// MockClient stubs the IClientServices interface.
type MockClient struct {
	ClientServices
	MethodCalled bool
}

// Delete is a mock that records that this method has been called.
func (m *MockClient) Delete(u string) (*roundtrip.Response, error) {
	m.MethodCalled = true
	return nil, nil
}

// Reset resets states to zero value.
func (m *MockClient) Reset() {
	m.MethodCalled = false
}

// MockAuthServiceClient stubs grpc AuthServiceClient interface.
type MockAuthServiceClient struct {
	proto.AuthServiceClient
}

// DeleteUser is a mock that returns an error.
func (m *MockAuthServiceClient) DeleteUser(ctx context.Context, in *proto.DeleteUserRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, trail.ToGRPC(trace.NotImplemented(""))
}

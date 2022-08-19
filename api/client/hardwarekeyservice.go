/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"context"

	"github.com/gravitational/trace/trail"

	"github.com/gravitational/teleport/api/client/proto"
)

// HardwareKeyServiceClient is a client for the HardwareKeyService, which runs on both the
// auth and proxy.
type HardwareKeyServiceClient struct {
	grpcClient proto.HardwareKeyServiceClient
}

// NewHardwareKeyServiceClient returns a new HardwareKeyServiceClient wrapping the given grpc
// client.
func NewHardwareKeyServiceClient(grpcClient proto.HardwareKeyServiceClient) *HardwareKeyServiceClient {
	return &HardwareKeyServiceClient{
		grpcClient: grpcClient,
	}
}

// AttestHardwarePrivateKey attests a hardware private key so that it
// will be trusted by the Auth server in subsequent calls.
func (c *HardwareKeyServiceClient) AttestHardwarePrivateKey(ctx context.Context, req *proto.AttestHardwarePrivateKeyRequest) error {
	_, err := c.grpcClient.AttestHardwarePrivateKey(ctx, req)
	return trail.FromGRPC(err)
}

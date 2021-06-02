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

package metadata

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/api/constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const versionKey = "version"

var (
	versionValue = constants.Version
	disabled     = false
	mu           sync.RWMutex
)

// SetVersion is used only for tests to set the version value that the client
// will send in the GRPC metadata.
func SetVersion(version string) {
	mu.Lock()
	defer mu.Unlock()
	versionValue = version
}

// DisabledIs disables (or enables) including metadata in GRPC requests. Used in tests.
func DisabledIs(metadataDisabled bool) {
	mu.Lock()
	defer mu.Unlock()
	disabled = metadataDisabled
}

func addMetadataToContext(ctx context.Context) context.Context {
	mu.RLock()
	defer mu.RUnlock()
	if disabled {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, versionKey, versionValue)
}

// StreamClientInterceptor intercepts a GRPC client stream call and adds metadata to the context
func StreamClientInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	ctx = addMetadataToContext(ctx)
	return streamer(ctx, desc, cc, method, opts...)
}

// UnaryClientInterceptor intercepts a GRPC client unary call and adds metadata to the context
func UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx = addMetadataToContext(ctx)
	return invoker(ctx, method, req, reply, cc, opts...)
}

// VersionFromContext can be called from a GRPC server method to return the
// client version that was added to the GRPC metadata by
// StreamClientInterceptor or UnaryClientInterceptor on the client.
func VersionFromContext(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	versionList, ok := md[versionKey]
	if !ok || len(versionList) != 1 {
		return "", false
	}
	return versionList[0], true
}

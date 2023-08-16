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

package client

import (
	"context"
	"flag"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t)

	tests := []struct {
		desc      string
		config    Config
		assertErr require.ErrorAssertionFunc
	}{{
		desc: "successfully dial tcp address.",
		config: Config{
			Addrs: []string{srv.Addr()},
			Credentials: []Credentials{
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: require.NoError,
	}, {
		desc: "synchronously dial addr/cred pairs and succeed with the 1 good pair.",
		config: Config{
			Addrs: []string{"bad addr", srv.Addr(), "bad addr"},
			Credentials: []Credentials{
				&tlsConfigCreds{nil},
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
				&tlsConfigCreds{nil},
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: require.NoError,
	}, {
		desc: "fail to dial with a bad address.",
		config: Config{
			DialTimeout: time.Second,
			Addrs:       []string{"bad addr"},
			Credentials: []Credentials{
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "all connection methods failed")
		},
	}, {
		desc: "fail to dial with no address or dialer.",
		config: Config{
			DialTimeout: time.Second,
			Credentials: []Credentials{
				&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
			},
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
			},
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "no connection methods found, try providing Dialer or Addrs in config")
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clt, err := srv.NewClient(ctx, WithConfig(tt.config))
			tt.assertErr(t, err)

			if err == nil {
				t.Cleanup(func() { require.NoError(t, clt.Close()) })
				// requests to the server should succeed.
				_, err = clt.Ping(ctx)
				require.NoError(t, err)
			}
		})
	}
}

func TestNewDialBackground(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// get listener but don't serve it yet.
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	addr := l.Addr().String()

	// Create client before the server is listening.
	clt, err := New(ctx, Config{
		DialInBackground: true,
		Addrs:            []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// requests to the server will result in a connection error.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	_, err = clt.Ping(cancelCtx)
	require.Error(t, err)

	// Start the server and wait for the client connection to be ready.
	startMockServerWithListener(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// requests to the server should succeed.
	_, err = clt.Ping(ctx)
	require.NoError(t, err)
}

func TestWaitForConnectionReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	addr := l.Addr().String()

	// Create client before the server is listening.
	clt, err := New(ctx, Config{
		DialInBackground: true,
		Addrs:            []string{addr},
		Credentials: []Credentials{
			&mockInsecureTLSCredentials{}, // TODO(Joerger) replace insecure credentials
		},
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO(Joerger) remove insecure dial option
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// WaitForConnectionReady should return false once the
	// context is canceled if the server isn't open to connections.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	require.Error(t, clt.waitForConnectionReady(cancelCtx))

	// WaitForConnectionReady should return nil if the server is open to connections.
	startMockServerWithListener(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// WaitForConnectionReady should return an error if the grpc connection is closed.
	require.NoError(t, clt.Close())
	require.Error(t, clt.waitForConnectionReady(ctx))
}

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t)

	testCases := map[string]struct {
		resourceType   string
		resourceStruct types.Resource
	}{
		"DatabaseServer": {
			resourceType:   types.KindDatabaseServer,
			resourceStruct: &types.DatabaseServerV3{},
		},
		"ApplicationServer": {
			resourceType:   types.KindAppServer,
			resourceStruct: &types.AppServerV3{},
		},
		"Node": {
			resourceType:   types.KindNode,
			resourceStruct: &types.ServerV2{},
		},
		"KubeServer": {
			resourceType:   types.KindKubeServer,
			resourceStruct: &types.KubernetesServerV3{},
		},
		"WindowsDesktop": {
			resourceType:   types.KindWindowsDesktop,
			resourceStruct: &types.WindowsDesktopV3{},
		},
	}

	// Create client
	clt, err := srv.NewClient(ctx)
	require.NoError(t, err)

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        10,
				ResourceType: test.resourceType,
			})
			require.NoError(t, err)
			require.NotEmpty(t, resp.NextKey)
			require.Len(t, resp.Resources, 10)
			require.IsType(t, test.resourceStruct, resp.Resources[0])

			// exceed the limit
			_, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        50,
				ResourceType: test.resourceType,
			})
			require.Error(t, err)
			require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())
		})
	}

	// Test a list with total count returned.
	resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType:   types.KindNode,
		Limit:          10,
		NeedTotalCount: true,
	})
	require.NoError(t, err)
	require.Equal(t, 50, resp.TotalCount)
}

func testGetResources[T types.ResourceWithLabels](t *testing.T, clt *Client, kind string) {
	ctx := context.Background()
	expectedResources, err := testResources[T](kind, defaults.Namespace)
	require.NoError(t, err)

	// Test listing everything at once errors with limit exceeded.
	_, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		Namespace:    defaults.Namespace,
		Limit:        int32(len(expectedResources)),
		ResourceType: kind,
	})
	require.Error(t, err)
	require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())

	// Test getting a page of resources
	page, err := GetResourcePage[T](ctx, clt, &proto.ListResourcesRequest{
		Namespace:      defaults.Namespace,
		ResourceType:   kind,
		NeedTotalCount: true,
	})
	require.NoError(t, err)
	require.Len(t, expectedResources, page.Total)
	require.Empty(t, cmp.Diff(expectedResources[:len(page.Resources)], page.Resources))

	// Test getting all resources by chunks to handle limit exceeded.
	resources, err := GetAllResources[T](ctx, clt, &proto.ListResourcesRequest{
		Namespace:    defaults.Namespace,
		ResourceType: kind,
	})
	require.NoError(t, err)
	require.Len(t, resources, len(expectedResources))
	require.Empty(t, cmp.Diff(expectedResources, resources))
}

func TestGetResources(t *testing.T) {
	t.Parallel()
	srv := startMockServer(t)

	// Create client
	clt, err := srv.NewClient(context.Background())
	require.NoError(t, err)

	t.Run("DatabaseServer", func(t *testing.T) {
		t.Parallel()
		testGetResources[types.DatabaseServer](t, clt, types.KindDatabaseServer)
	})

	t.Run("ApplicationServer", func(t *testing.T) {
		t.Parallel()
		testGetResources[types.AppServer](t, clt, types.KindAppServer)
	})

	t.Run("Node", func(t *testing.T) {
		t.Parallel()
		testGetResources[types.Server](t, clt, types.KindNode)
	})

	t.Run("KubeServer", func(t *testing.T) {
		t.Parallel()
		testGetResources[types.KubeServer](t, clt, types.KindKubeServer)
	})

	t.Run("WindowsDesktop", func(t *testing.T) {
		t.Parallel()
		testGetResources[types.WindowsDesktop](t, clt, types.KindWindowsDesktop)
	})

	t.Run("AppServerAndSAMLIdPServiceProvider", func(t *testing.T) {
		t.Parallel()
		testGetResources[types.AppServerOrSAMLIdPServiceProvider](t, clt, types.KindAppOrSAMLIdPServiceProvider)
	})
}

func TestGetResourcesWithFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t)

	// Create client
	clt, err := srv.NewClient(ctx)
	require.NoError(t, err)

	testCases := map[string]struct {
		resourceType string
	}{
		"DatabaseServer": {
			resourceType: types.KindDatabaseServer,
		},
		"ApplicationServer": {
			resourceType: types.KindAppServer,
		},
		"Node": {
			resourceType: types.KindNode,
		},
		"KubeServer": {
			resourceType: types.KindKubeServer,
		},
		"WindowsDesktop": {
			resourceType: types.KindWindowsDesktop,
		},
		"AppAndIdPServiceProvider": {
			resourceType: types.KindAppOrSAMLIdPServiceProvider,
		},
	}

	for name, test := range testCases {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expectedResources, err := testResources[types.ResourceWithLabels](test.resourceType, defaults.Namespace)
			require.NoError(t, err)

			// Test listing everything at once errors with limit exceeded.
			_, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				Limit:        int32(len(expectedResources)),
				ResourceType: test.resourceType,
			})
			require.Error(t, err)
			require.IsType(t, &trace.LimitExceededError{}, err.(*trace.TraceErr).OrigError())

			// Test getting all resources by chunks to handle limit exceeded.
			resources, err := GetResourcesWithFilters(ctx, clt, proto.ListResourcesRequest{
				Namespace:    defaults.Namespace,
				ResourceType: test.resourceType,
			})
			require.NoError(t, err)
			require.Len(t, resources, len(expectedResources))
			require.Empty(t, cmp.Diff(expectedResources, resources))
		})
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Verbose() {
		logrus.SetLevel(logrus.DebugLevel)
	}
	os.Exit(m.Run())
}

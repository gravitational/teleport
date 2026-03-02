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
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	trustv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

type pingService struct {
	proto.UnimplementedAuthServiceServer
	userAgentFromLastCallValue atomic.Value
}

func (s *pingService) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	s.userAgentFromLastCallValue.Store(metadata.UserAgentFromContext(ctx))
	return &proto.PingResponse{}, nil
}

func (s *pingService) userAgentFromLastCall() string {
	if userAgent, ok := s.userAgentFromLastCallValue.Load().(string); ok {
		return userAgent
	}
	return ""
}

func TestDialTimeout(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc    string
		timeout time.Duration
	}{
		{
			desc:    "dial timeout set to valid value",
			timeout: 500 * time.Millisecond,
		},
		{
			desc:    "defaults prevent infinite timeout",
			timeout: 0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				credentials := []Credentials{LoadTLS(&tls.Config{})}

				// CheckAndSetDefaults may modify the DialTimeout. Create a throwaway config
				// to get the actual timeout that will be used by the client.
				timeout := func() time.Duration {
					cfg := Config{
						Credentials: credentials,
						DialTimeout: tt.timeout,
					}

					require.NoError(t, cfg.CheckAndSetDefaults())
					return cfg.DialTimeout
				}()

				// Create a client that will never connect to anything. All dial attempts will sleep
				// indefinitely.
				cfg := Config{
					DialTimeout: tt.timeout,
					Credentials: credentials,
					Dialer: ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
						select {
						case <-time.After(24 * time.Hour):
							return nil, trace.ConnectionProblem(nil, "dial timeout")
						case <-ctx.Done():
							return nil, ctx.Err()
						}
					}),
				}

				errChan := make(chan error, 1)
				go func() {
					// try to create a client - this will time out after the DialTimeout threshold is exceeded
					_, err := New(t.Context(), cfg)
					errChan <- err
				}()

				// wait for the client creation to be blocked
				synctest.Wait()

				// advance the clock so that the timeout kicks in
				time.Sleep(timeout)
				synctest.Wait()

				// validate the client creation to fail due to the timeout being enforced.
				err := <-errChan
				require.Error(t, err)
				require.ErrorContains(t, err, context.DeadlineExceeded.Error())
			})
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t, mockServices{auth: &pingService{}})

	tests := []struct {
		desc         string
		modifyConfig func(*Config)
		assertErr    require.ErrorAssertionFunc
	}{{
		desc:         "successfully dial tcp address.",
		modifyConfig: func(c *Config) { /* noop */ },
		assertErr:    require.NoError,
	}, {
		desc: "synchronously dial addr/cred pairs and succeed with the 1 good pair.",
		modifyConfig: func(c *Config) {
			c.Addrs = append(c.Addrs, "bad addr", "bad addr")
			c.Credentials = append([]Credentials{&tlsConfigCreds{nil}, &tlsConfigCreds{nil}}, c.Credentials...)
		},
		assertErr: require.NoError,
	}, {
		desc: "fail to dial with a bad address.",
		modifyConfig: func(c *Config) {
			c.Addrs = []string{"bad addr"}
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.ErrorContains(t, err, "all connection methods failed")
		},
	}, {
		desc: "fail to dial with no address or dialer.",
		modifyConfig: func(c *Config) {
			c.Addrs = nil
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.ErrorContains(t, err, "no connection methods found, try providing Dialer or Addrs in config")
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cfg := srv.clientCfg()
			tt.modifyConfig(&cfg)

			clt, err := New(ctx, cfg)
			tt.assertErr(t, err)
			if err != nil {
				return
			}

			// Requests to the server should succeed.
			_, err = clt.Ping(ctx)
			assert.NoError(t, err, "Ping failed")
			assert.NoError(t, clt.Close(), "Close failed")
		})
	}
}

func TestNewDialBackground(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a server but don't serve it yet.
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	addr := l.Addr().String()
	ping := &pingService{}
	srv := newMockServer(t, addr, mockServices{auth: ping})

	// Create client before the server is listening.
	cfg := srv.clientCfg()
	cfg.DialInBackground = true
	cfg.DialOpts = append(cfg.DialOpts, metadata.WithUserAgentFromTeleportComponent("api-client-test"))
	clt, err := New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// requests to the server will result in a connection error.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	_, err = clt.Ping(cancelCtx)
	require.Error(t, err)

	// Server the listener and wait for the client connection to be ready.
	srv.serve(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// requests to the server should succeed.
	_, err = clt.Ping(ctx)
	require.NoError(t, err)

	// Verify user agent.
	expectUserAgentPrefix := fmt.Sprintf("api-client-test/%v grpc-go/", api.Version)
	require.True(t, strings.HasPrefix(ping.userAgentFromLastCall(), expectUserAgentPrefix))
}

func TestWaitForConnectionReady(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a server but don't serve it yet.
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	addr := l.Addr().String()
	srv := newMockServer(t, addr, mockServices{auth: &proto.UnimplementedAuthServiceServer{}})

	// Create client before the server is listening.
	cfg := srv.clientCfg()
	cfg.DialInBackground = true
	clt, err := New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	// WaitForConnectionReady should return an error once the
	// context is canceled if the server isn't open to connections.
	cancelCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	require.Error(t, clt.waitForConnectionReady(cancelCtx))

	// WaitForConnectionReady should return nil if the server is open to connections.
	srv.serve(t, l)
	require.NoError(t, clt.waitForConnectionReady(ctx))

	// WaitForConnectionReady should return an error if the grpc connection is closed.
	require.NoError(t, clt.Close())
	require.Error(t, clt.waitForConnectionReady(ctx))
}

type listResourcesService struct {
	proto.UnimplementedAuthServiceServer
}

func (s *listResourcesService) ListResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	expectedResources, err := testResources[types.ResourceWithLabels](req.ResourceType, req.Namespace)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	resp := &proto.ListResourcesResponse{
		Resources:  make([]*proto.PaginatedResource, 0, len(expectedResources)),
		TotalCount: int32(len(expectedResources)),
	}

	var (
		takeResources    = req.StartKey == ""
		lastResourceName string
	)
	for _, resource := range expectedResources {
		if resource.GetName() == req.StartKey {
			takeResources = true
			continue
		}

		if !takeResources {
			continue
		}

		var protoResource *proto.PaginatedResource
		switch req.ResourceType {
		case types.KindDatabaseServer:
			database, ok := resource.(*types.DatabaseServerV3)
			if !ok {
				return nil, trace.Errorf("database server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}}
		case types.KindAppServer:
			app, ok := resource.(*types.AppServerV3)
			if !ok {
				return nil, trace.Errorf("application server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_AppServer{AppServer: app}}
		case types.KindNode:
			srv, ok := resource.(*types.ServerV2)
			if !ok {
				return nil, trace.Errorf("node has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: srv}}
		case types.KindKubeServer:
			srv, ok := resource.(*types.KubernetesServerV3)
			if !ok {
				return nil, trace.Errorf("kubernetes server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubernetesServer{KubernetesServer: srv}}
		case types.KindWindowsDesktop:
			desktop, ok := resource.(*types.WindowsDesktopV3)
			if !ok {
				return nil, trace.Errorf("windows desktop has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_WindowsDesktop{WindowsDesktop: desktop}}
		case types.KindSAMLIdPServiceProvider:
			samlSP, ok := resource.(*types.SAMLIdPServiceProviderV1)
			if !ok {
				return nil, trace.Errorf("SAML IdP service provider has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_SAMLIdPServiceProvider{SAMLIdPServiceProvider: samlSP}}
		}
		resp.Resources = append(resp.Resources, protoResource)
		lastResourceName = resource.GetName()
		if len(resp.Resources) == int(req.Limit) {
			break
		}
	}

	if len(resp.Resources) != len(expectedResources) {
		resp.NextKey = lastResourceName
	}

	return resp, nil
}

const fiveMBNode = "fiveMBNode"

func testResources[T types.ResourceWithLabels](resourceType, namespace string) ([]T, error) {
	size := 50
	// Artificially make each node ~ 100KB to force
	// ListResources to fail with chunks of >= 40.
	labelSize := 100000
	resources := make([]T, 0, size)

	switch resourceType {
	case types.KindDatabaseServer:
		for i := 0; i < size; i++ {
			resource, err := types.NewDatabaseServerV3(types.Metadata{
				Name: fmt.Sprintf("db-%d", i),
				Labels: map[string]string{
					"label": string(make([]byte, labelSize)),
				},
			}, types.DatabaseServerSpecV3{
				Hostname: "localhost",
				HostID:   fmt.Sprintf("host-%d", i),
				Database: &types.DatabaseV3{
					Metadata: types.Metadata{
						Name: fmt.Sprintf("db-%d", i),
					},
					Spec: types.DatabaseSpecV3{
						Protocol: types.DatabaseProtocolPostgreSQL,
						URI:      "localhost",
					},
				},
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resources = append(resources, any(resource).(T))
		}
	case types.KindAppServer:
		for i := 0; i < size; i++ {
			app, err := types.NewAppV3(types.Metadata{
				Name: fmt.Sprintf("app-%d", i),
			}, types.AppSpecV3{
				URI: "localhost",
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resource, err := types.NewAppServerV3(types.Metadata{
				Name: fmt.Sprintf("app-%d", i),
				Labels: map[string]string{
					"label": string(make([]byte, labelSize)),
				},
			}, types.AppServerSpecV3{
				HostID: fmt.Sprintf("host-%d", i),
				App:    app,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resources = append(resources, any(resource).(T))
		}
	case types.KindNode:
		for i := 0; i < size; i++ {
			nodeLabelSize := labelSize
			if namespace == fiveMBNode && i == 0 {
				// Artificially make a node ~ 5MB to force
				// ListNodes to fail regardless of chunk size.
				nodeLabelSize = 5000000
			}

			var err error
			resource, err := types.NewServerWithLabels(fmt.Sprintf("node-%d", i), types.KindNode, types.ServerSpecV2{},
				map[string]string{
					"label": string(make([]byte, nodeLabelSize)),
				},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resources = append(resources, any(resource).(T))
		}
	case types.KindKubeServer:
		for i := 0; i < size; i++ {
			var err error
			name := fmt.Sprintf("kube-service-%d", i)
			kube, err := types.NewKubernetesClusterV3(types.Metadata{
				Name:   name,
				Labels: map[string]string{"name": name},
			},
				types.KubernetesClusterSpecV3{},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			resource, err := types.NewKubernetesServerV3(
				types.Metadata{
					Name: name,
					Labels: map[string]string{
						"label": string(make([]byte, labelSize)),
					},
				},
				types.KubernetesServerSpecV3{
					HostID:  fmt.Sprintf("host-%d", i),
					Cluster: kube,
				},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resources = append(resources, any(resource).(T))
		}
	case types.KindWindowsDesktop:
		for i := 0; i < size; i++ {
			var err error
			name := fmt.Sprintf("windows-desktop-%d", i)
			resource, err := types.NewWindowsDesktopV3(
				name,
				map[string]string{"label": string(make([]byte, labelSize))},
				types.WindowsDesktopSpecV3{
					Addr:   "_",
					HostID: "_",
				})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resources = append(resources, any(resource).(T))
		}
	case types.KindSAMLIdPServiceProvider:
		for i := 0; i < size; i++ {
			name := fmt.Sprintf("saml-app-%d", i)
			spResource, err := types.NewSAMLIdPServiceProvider(
				types.Metadata{
					Name: name, Labels: map[string]string{
						"label": string(make([]byte, labelSize)),
					},
				},
				types.SAMLIdPServiceProviderSpecV1{
					EntityID: name,
					ACSURL:   name,
				},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resources = append(resources, any(spResource).(T))
		}
	default:
		return nil, trace.Errorf("unsupported resource type %s", resourceType)
	}

	return resources, nil
}

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t, mockServices{auth: &listResourcesService{}})

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
		"SAMLIdPServiceProvider": {
			resourceType:   types.KindSAMLIdPServiceProvider,
			resourceStruct: &types.SAMLIdPServiceProviderV1{},
		},
	}

	// Create client
	clt, err := New(ctx, srv.clientCfg())
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
			require.True(t, trace.IsLimitExceeded(err), "trace.IsLimitExceeded failed: err=%v (%T)", err, trace.Unwrap(err))
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
	require.True(t, trace.IsLimitExceeded(err), "trace.IsLimitExceeded failed: err=%v (%T)", err, trace.Unwrap(err))

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
	ctx := context.Background()
	srv := startMockServer(t, mockServices{auth: &listResourcesService{}})

	// Create client
	clt, err := New(ctx, srv.clientCfg())
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

	t.Run("SAMLIdPServiceProvider", func(t *testing.T) {
		t.Parallel()
		testGetResources[types.SAMLIdPServiceProvider](t, clt, types.KindSAMLIdPServiceProvider)
	})
}

func TestGetResourcesWithFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := startMockServer(t, mockServices{auth: &listResourcesService{}})

	// Create client
	clt, err := New(ctx, srv.clientCfg())
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
		"SAMLIdPServiceProvider": {
			resourceType: types.KindSAMLIdPServiceProvider,
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
			require.True(t, trace.IsLimitExceeded(err), "trace.IsLimitExceeded failed: err=%v (%T)", err, trace.Unwrap(err))

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

type fakeUnifiedResourcesClient struct {
	resp *proto.ListUnifiedResourcesResponse
	err  error
}

func (f fakeUnifiedResourcesClient) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	return f.resp, f.err
}

// TestGetUnifiedResourcesWithLogins validates that any logins provided
// in a [proto.PaginatedResource] are correctly parsed and applied to
// the corresponding [types.EnrichedResource].
func TestGetUnifiedResourcesWithLogins(t *testing.T) {
	ctx := context.Background()

	clt := fakeUnifiedResourcesClient{
		resp: &proto.ListUnifiedResourcesResponse{
			Resources: []*proto.PaginatedResource{
				{
					Resource: &proto.PaginatedResource_Node{Node: &types.ServerV2{}},
					Logins:   []string{"alice", "bob"},
				},
				{
					Resource: &proto.PaginatedResource_WindowsDesktop{WindowsDesktop: &types.WindowsDesktopV3{}},
					Logins:   []string{"llama"},
				},
				{
					Resource: &proto.PaginatedResource_AppServer{AppServer: &types.AppServerV3{}},
					Logins:   []string{"llama"},
				},
			},
		},
	}

	resources, _, err := GetUnifiedResourcePage(ctx, clt, &proto.ListUnifiedResourcesRequest{
		SortBy: types.SortBy{
			IsDesc: false,
			Field:  types.ResourceSpecHostname,
		},
		IncludeLogins: true,
	})
	require.NoError(t, err)

	require.Len(t, resources, len(clt.resp.Resources))

	for _, enriched := range resources {
		switch enriched.ResourceWithLabels.(type) {
		case *types.ServerV2:
			assert.Equal(t, enriched.Logins, clt.resp.Resources[0].Logins)
		case *types.WindowsDesktopV3:
			assert.Equal(t, enriched.Logins, clt.resp.Resources[1].Logins)
		case *types.AppServerV3:
			assert.Equal(t, enriched.Logins, clt.resp.Resources[2].Logins)
		}
	}
}

func TestUploadEncryptedRecording(t *testing.T) {
	ctx := t.Context()

	recordingEncryptionService := &uploadRecordingService{
		uploads: make(map[string][]*recordingencryptionv1.Part),
	}
	srv := startMockServer(t, mockServices{recordingEncryption: recordingEncryptionService})
	parts := [][]byte{
		[]byte("123"),
		[]byte("456"),
		[]byte("789"),
	}
	partIter := func(yield func([]byte, error) bool) {
		for _, part := range parts {
			if part == nil {
				if !yield(nil, errors.New("invalid part")) {
					return
				}
			} else {
				if !yield(part, nil) {
					return
				}
			}
		}
	}

	clt, err := New(ctx, srv.clientCfg())
	require.NoError(t, err)

	sessionID, err := uuid.NewV7()
	require.NoError(t, err)
	err = clt.UploadEncryptedRecording(ctx, sessionID.String(), partIter)
	require.NoError(t, err)

	uploaded := recordingEncryptionService.uploads[sessionID.String()]
	require.Len(t, uploaded, len(parts))
	for idx, part := range uploaded {
		// uploaded part numbers should increment starting with 1
		require.Equal(t, int64(idx+1), part.PartNumber)
	}
}

type uploadRecordingService struct {
	recordingencryptionv1.UnimplementedRecordingEncryptionServiceServer

	uploads map[string][]*recordingencryptionv1.Part
}

func (s *uploadRecordingService) CreateUpload(ctx context.Context, req *recordingencryptionv1.CreateUploadRequest) (*recordingencryptionv1.CreateUploadResponse, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.uploads[req.SessionId] = []*recordingencryptionv1.Part{}
	return &recordingencryptionv1.CreateUploadResponse{
		Upload: &recordingencryptionv1.Upload{
			UploadId:  id.String(),
			SessionId: req.SessionId,
		},
	}, nil
}

func (s *uploadRecordingService) UploadPart(ctx context.Context, req *recordingencryptionv1.UploadPartRequest) (*recordingencryptionv1.UploadPartResponse, error) {
	sessionID := req.GetUpload().GetSessionId()
	parts, ok := s.uploads[sessionID]
	if !ok {
		return nil, trace.Errorf("no upload found for %s", sessionID)
	}
	part := &recordingencryptionv1.Part{
		PartNumber: req.PartNumber,
	}
	s.uploads[sessionID] = append(parts, part)
	return &recordingencryptionv1.UploadPartResponse{
		Part: part,
	}, nil
}

func (s *uploadRecordingService) CompleteUpload(ctx context.Context, req *recordingencryptionv1.CompleteUploadRequest) (*recordingencryptionv1.CompleteUploadResponse, error) {
	sessionID := req.GetUpload().GetSessionId()
	parts, ok := s.uploads[sessionID]
	if !ok {
		return nil, trace.Errorf("no upload found for %s", sessionID)
	}
	if len(parts) != len(req.GetParts()) {
		return nil, errors.New("parts reported as uploaded is not the ")
	}
	for _, part := range req.GetParts() {
		uploaded := parts[part.PartNumber-1]
		if uploaded.PartNumber != part.PartNumber {
			return nil, fmt.Errorf("expected part %d in place of %d", part.PartNumber, uploaded.PartNumber)
		}
	}
	return nil, nil
}

func TestWindowsCAFallback(t *testing.T) {
	t.Parallel()

	const clusterName = "zarquon"
	userCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert:    []byte(`unused by test`),
					Key:     []byte(`unused by test`),
					KeyType: 0, // unused by test
					CRL:     []byte(`ceci n'est pas une CRL`),
				},
			},
		},
	})
	require.NoError(t, err)
	userCAV2, ok := userCA.(*types.CertAuthorityV2)
	require.True(t, ok, "%T is not of type %T", userCA, userCAV2)

	mockService := &mockPreWindowsService{
		currentCluster: clusterName,
		cas: []*types.CertAuthorityV2{
			userCAV2,
		},
	}

	ctx := t.Context()
	server := startMockServer(t, mockServices{
		auth:          mockService.Auth(),
		clusterConfig: mockService,
		trust:         mockService,
	})

	c, err := New(ctx, server.clientCfg())
	require.NoError(t, err)

	id := types.CertAuthID{
		Type:       types.WindowsCA,
		DomainName: clusterName,
	}
	const loadKeys = false

	t.Run("list", func(t *testing.T) {
		// Don't t.Parallel(), let this all run in sequence because of "listHardFails".
		t.Cleanup(func() {
			mockService.listHardFails.Store(false)
		})

		var name string
		for _, val := range []bool{false, true} {
			if val {
				name = "unknown authority"
			} else {
				name = "empty response"
			}
			mockService.listHardFails.Store(val)
			t.Run(name, func(t *testing.T) {
				// Don't t.Parallel().

				got, err := c.GetCertAuthorities(ctx, id.Type, loadKeys)
				require.NoError(t, err)

				want := []types.CertAuthority{userCA}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("GetCertAuthorities mismatch (-want +got)\n%s", diff)
				}
			})
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		got, err := c.GetCertAuthority(ctx, id, loadKeys)
		require.NoError(t, err)
		if diff := cmp.Diff(userCA, got); diff != "" {
			t.Errorf("GetCertAuthority mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("crl", func(t *testing.T) {
		t.Parallel()

		got, err := c.GenerateCertAuthorityCRL(ctx, &proto.CertAuthorityRequest{
			Type: id.Type,
		})
		require.NoError(t, err)

		want := &proto.CRL{
			CRL: userCA.GetActiveKeys().TLS[0].CRL,
		}
		require.NotEmpty(t, want.CRL) // sanity check
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("GenerateCertAuthorityCRL mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("not founds", func(t *testing.T) {
		t.Parallel()

		// Get with unknown domain.
		id2 := id
		id2.DomainName = "unknown"
		_, err := c.GetCertAuthority(ctx, id2, loadKeys)
		assert.ErrorContains(t, err, "not found", "GetCertAuthority error mismatch")

		// Get with unknown type.
		id2 = id
		id2.Type = types.DatabaseCA
		_, err = c.GetCertAuthority(ctx, id2, loadKeys)
		assert.ErrorContains(t, err, "not found", "GetCertAuthority error mismatch")

		// List with unknown type.
		resp, err := c.GetCertAuthorities(ctx, id2.Type, loadKeys)
		require.NoError(t, err)
		assert.Empty(t, resp, "GetCertAuthorities returned unexpected CAs")

		// CRL with unknown type.
		_, err = c.GenerateCertAuthorityCRL(ctx, &proto.CertAuthorityRequest{
			Type: id2.Type,
		})
		assert.ErrorContains(t, err, "not found")
	})
}

type mockPreWindowsAuthService struct {
	proto.UnimplementedAuthServiceServer

	// delegates implementations to mockPreWindowsTrustService.
	// The "AuthService" and "TrustService" interfaces clash, so we can't embed
	// them both in a single type.
	impl *mockPreWindowsService
}

func (s *mockPreWindowsAuthService) GenerateCertAuthorityCRL(
	ctx context.Context, req *proto.CertAuthorityRequest) (*proto.CRL, error) {
	return s.impl.GenerateCertAuthorityCRL(ctx, req)
}

type mockPreWindowsService struct {
	clusterconfigv1.UnimplementedClusterConfigServiceServer
	trustv1.UnimplementedTrustServiceServer

	currentCluster string
	cas            []*types.CertAuthorityV2

	// If true GetCertAuthorities hard-fails, instead of an empty response.
	listHardFails atomic.Bool
}

func (s *mockPreWindowsService) Auth() proto.AuthServiceServer {
	return &mockPreWindowsAuthService{impl: s}
}

// GenerateCertAuthorityCRL, as implemented here, doesn't generate a CRL but
// instead returns the CRL of the first active TLS key pair within the CA.
// The CRL is never interpreted beyond checking for a non-empty slice, so it
// works with fake data.
func (s *mockPreWindowsService) GenerateCertAuthorityCRL(
	ctx context.Context, req *proto.CertAuthorityRequest) (*proto.CRL, error) {
	ca, err := s.GetCertAuthority(ctx, &trustv1.GetCertAuthorityRequest{
		Type:       string(req.Type),
		Domain:     s.currentCluster,
		IncludeKey: false,
	})
	if err != nil {
		return nil, err
	}

	tlsKPs := ca.GetActiveKeys().TLS
	if len(tlsKPs) == 0 {
		return nil, trace.BadParameter("no TLS key pairs found within CA")
	}
	tlsKP := tlsKPs[0]

	if len(tlsKP.CRL) == 0 {
		return nil, trace.BadParameter("no CRL found in the first TLS key pair of the CA")
	}
	return &proto.CRL{
		CRL: tlsKP.CRL,
	}, nil
}

func (s *mockPreWindowsService) GetCertAuthority(
	ctx context.Context,
	req *trustv1.GetCertAuthorityRequest,
) (*types.CertAuthorityV2, error) {
	if err := s.failIfWindowsCA(req.Type); err != nil {
		return nil, err
	}

	for _, ca := range s.cas {
		if ca.Spec.Type == types.CertAuthType(req.Type) && ca.Spec.ClusterName == req.Domain {
			return ca.Clone().(*types.CertAuthorityV2), nil
		}
	}
	return nil, trace.NotFound("ca not found")
}

func (s *mockPreWindowsService) GetCertAuthorities(
	ctx context.Context,
	req *trustv1.GetCertAuthoritiesRequest,
) (*trustv1.GetCertAuthoritiesResponse, error) {
	if s.listHardFails.Load() {
		if err := s.failIfWindowsCA(req.Type); err != nil {
			return nil, err
		}
	}

	resp := &trustv1.GetCertAuthoritiesResponse{}
	for _, ca := range s.cas {
		if ca.Spec.Type == types.CertAuthType(req.Type) {
			resp.CertAuthoritiesV2 = append(resp.CertAuthoritiesV2, ca.Clone().(*types.CertAuthorityV2))
		}
	}
	return resp, nil
}

func (s *mockPreWindowsService) GetClusterName(
	ctx context.Context,
	req *clusterconfigv1.GetClusterNameRequest,
) (*types.ClusterNameV2, error) {
	return &types.ClusterNameV2{
		Spec: types.ClusterNameSpecV2{
			ClusterName: s.currentCluster,
		},
	}, nil
}

func (s *mockPreWindowsService) failIfWindowsCA(caType string) error {
	// Mimic a types.CertAuthorityType.Check() failure.
	if caType == string(types.WindowsCA) {
		return trace.BadParameter(`%q authority type is not supported`, caType)
	}
	return nil
}

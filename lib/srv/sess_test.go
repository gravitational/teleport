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

package srv

import (
	"context"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestParseAccessRequestIDs(t *testing.T) {
	testCases := []struct {
		input     string
		comment   string
		result    []string
		assertErr require.ErrorAssertionFunc
	}{
		{
			input:     `{"access_requests":["1a7483e0-575a-4bd1-9faa-022500a49325","30b344f5-d1ba-49fc-b2aa-b04234d0a4ec"]}`,
			comment:   "complete valid input",
			assertErr: require.NoError,
			result:    []string{"1a7483e0-575a-4bd1-9faa-022500a49325", "30b344f5-d1ba-49fc-b2aa-b04234d0a4ec"},
		},
		{
			input:     `{"access_requests":["1a7483e0-575a-4bd1-9faa-022500a49325","30b344f5-d1ba-49fc-b2aa"]}`,
			comment:   "invalid uuid",
			assertErr: require.Error,
			result:    nil,
		},
		{
			input:     `{"access_requests":[nil,"30b344f5-d1ba-49fc-b2aa-b04234d0a4ec"]}`,
			comment:   "invalid value, value in slice is nil",
			assertErr: require.Error,
			result:    nil,
		},
		{
			input:     `{"access_requests":nil}`,
			comment:   "invalid value, whole value is nil",
			assertErr: require.Error,
			result:    nil,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.comment, func(t *testing.T) {
			out, err := parseAccessRequestIDs(tt.input)
			tt.assertErr(t, err)
			require.Equal(t, out, tt.result)
		})
	}

}

type mockServer struct {
	events.StreamEmitter
}

// ID is the unique ID of the server.
func (m *mockServer) ID() string {
	return "test"
}

// HostUUID is the UUID of the underlying host. For the forwarding
// server this is the proxy the forwarding server is running in.
func (m *mockServer) HostUUID() string {
	return "test"
}

// GetNamespace returns the namespace the server was created in.
func (m *mockServer) GetNamespace() string {
	return "test"
}

// AdvertiseAddr is the publicly addressable address of this server.
func (m *mockServer) AdvertiseAddr() string {
	return "test"
}

// Component is the type of server, forwarding or regular.
func (m *mockServer) Component() string {
	return teleport.ComponentNode
}

// PermitUserEnvironment returns if reading environment variables upon
// startup is allowed.
func (m *mockServer) PermitUserEnvironment() bool {
	return false
}

// GetAccessPoint returns an AccessPoint for this cluster.
func (m *mockServer) GetAccessPoint() AccessPoint {
	return nil
}

// GetSessionServer returns a session server.
func (m *mockServer) GetSessionServer() rsession.Service {
	return nil
}

// GetDataDir returns data directory of the server
func (m *mockServer) GetDataDir() string {
	return "test"
}

// GetPAM returns PAM configuration for this server.
func (m *mockServer) GetPAM() (*pam.Config, error) {
	return nil, nil
}

// GetClock returns a clock setup for the server
func (m *mockServer) GetClock() clockwork.Clock {
	return clockwork.NewRealClock()
}

// GetInfo returns a services.Server that represents this server.
func (m *mockServer) GetInfo() types.Server {
	return nil
}

// UseTunnel used to determine if this node has connected to this cluster
// using reverse tunnel.
func (m *mockServer) UseTunnel() bool {
	return false
}

// GetBPF returns the BPF service used for enhanced session recording.
func (m *mockServer) GetBPF() bpf.BPF {
	return nil
}

// GetRestrictedSessionManager returns the manager for restricting user activity
func (m *mockServer) GetRestrictedSessionManager() restricted.Manager {
	return nil
}

// Context returns server shutdown context
func (m *mockServer) Context() context.Context {
	return context.Background()
}

// GetUtmpPath returns the path of the user accounting database and log. Returns empty for system defaults.
func (m *mockServer) GetUtmpPath() (utmp, wtmp string) {
	return "test", "test"
}

// GetLockWatcher gets the server's lock watcher.
func (m *mockServer) GetLockWatcher() *services.LockWatcher {
	return nil
}

func TestSession_newRecorder(t *testing.T) {
	proxyRecording, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)

	proxyRecordingSync, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxySync,
	})
	require.NoError(t, err)

	nodeRecording, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtNode,
	})
	require.NoError(t, err)

	nodeRecordingSync, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtNodeSync,
	})
	require.NoError(t, err)

	logger := logrus.WithFields(logrus.Fields{
		trace.Component: teleport.ComponentAuth,
	})

	cases := []struct {
		desc         string
		sess         *session
		sctx         *ServerContext
		errAssertion require.ErrorAssertionFunc
		recAssertion require.ValueAssertionFunc
	}{
		{
			desc: "discard-stream-when-proxy-recording",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
					srv: &mockServer{},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecording,
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				_, ok := i.(*events.DiscardStream)
				require.True(t, ok)
			},
		},
		{
			desc: "discard-stream--when-proxy-sync-recording",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
					srv: &mockServer{},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecordingSync,
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				_, ok := i.(*events.DiscardStream)
				require.True(t, ok)
			},
		},
		{
			desc: "err-new-streamer-fails",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
					srv: &mockServer{},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: nodeRecording,
				srv:                    &mockServer{},
			},
			errAssertion: require.Error,
			recAssertion: require.Nil,
		},
		{
			desc: "err-new-audit-writer-fails",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
					srv: &mockServer{},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: nodeRecordingSync,
				srv:                    &mockServer{},
			},
			errAssertion: require.Error,
			recAssertion: require.Nil,
		},
		{
			desc: "audit-writer",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
					srv: &mockServer{},
				},
			},
			sctx: &ServerContext{
				ClusterName:            "test",
				SessionRecordingConfig: nodeRecordingSync,
				srv: &mockServer{
					StreamEmitter: &events.DiscardEmitter{},
				},
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				aw, ok := i.(*events.AuditWriter)
				require.True(t, ok)
				require.NoError(t, aw.Close(context.Background()))
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			rec, err := newRecorder(tt.sess, tt.sctx)
			tt.errAssertion(t, err)
			tt.recAssertion(t, rec)
		})
	}
}

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

package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/services"
	libsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type Suite struct {
	clock        clockwork.FakeClock
	dataDir      string
	authServer   *auth.TestAuthServer
	tlsServer    *auth.TestTLSServer
	authClient   *auth.Client
	appServer    *Server
	hostCertPool *x509.CertPool

	hostUUID              string
	closeContext          context.Context
	closeFunc             context.CancelFunc
	message               string
	hostport              string
	testhttp              *httptest.Server
	clientCertificate     tls.Certificate
	awsConsoleCertificate tls.Certificate

	user types.User
	role types.Role
}

func (s *Suite) TearDown(t *testing.T) {
	err := s.appServer.Close()
	require.NoError(t, err)

	err = s.authClient.Close()
	require.NoError(t, err)

	s.testhttp.Close()

	err = s.tlsServer.Auth().DeleteAllAppServers(s.closeContext, defaults.Namespace)
	require.NoError(t, err)

	s.closeFunc()

	err = s.tlsServer.Close()
	require.NoError(t, err)
}

type suiteConfig struct {
	// ResourceMatchers are resource watcher matchers.
	ResourceMatchers []services.ResourceMatcher
	// OnReconcile sets app resource reconciliation callback.
	OnReconcile func(types.Apps)
	// Apps are the apps to configure.
	Apps types.Apps
	// ServerStreamer is the auth server audit events streamer.
	ServerStreamer events.Streamer
	// CloudImporter will use the given cloud importer for the app server.
	CloudImporter labels.Importer
	// AppLabels are the labels assigned to the application.
	AppLabels map[string]string
	// RoleAppLabels are the labels set to allow for the user role.
	RoleAppLabels types.Labels
}

func SetUpSuite(t *testing.T) *Suite {
	return SetUpSuiteWithConfig(t, suiteConfig{})
}

func SetUpSuiteWithConfig(t *testing.T, config suiteConfig) *Suite {
	s := &Suite{}

	s.clock = clockwork.NewFakeClock()
	s.dataDir = t.TempDir()
	s.hostUUID = uuid.New().String()

	var err error
	// Create Auth Server.
	s.authServer, err = auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "root.example.com",
		Dir:         s.dataDir,
		Clock:       s.clock,
		Streamer:    config.ServerStreamer,
	})
	require.NoError(t, err)
	t.Cleanup(func() { s.authServer.Close() })

	if config.ServerStreamer != nil {
		err = s.authServer.AuthServer.SetSessionRecordingConfig(s.closeContext, &types.SessionRecordingConfigV2{
			Spec: types.SessionRecordingConfigSpecV2{Mode: types.RecordAtNodeSync},
		})
		require.NoError(t, err)
	}

	s.tlsServer, err = s.authServer.NewTestTLSServer()
	require.NoError(t, err)

	t.Cleanup(func() {
		s.tlsServer.Close()
	})

	// Set up the host cert pool.
	rootCA, err := s.tlsServer.Auth().GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: "root.example.com",
	}, false)
	require.NoError(t, err)
	s.hostCertPool, err = services.CertPool(rootCA)
	require.NoError(t, err)

	roleAppLabels := config.RoleAppLabels
	// Default to something that matches the staticLabels package level variable.
	if roleAppLabels == nil {
		roleAppLabels = types.Labels{
			"bar": []string{"baz"},
		}
	}

	// Grant the user's role access to the application label "bar: baz".
	s.role = &types.RoleV5{
		Metadata: types.Metadata{
			Name: "foo",
		},
		Version: types.V5,
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				AppLabels:   roleAppLabels,
				AWSRoleARNs: []string{"readonly"},
			},
		},
	}
	// Create user for regular tests.
	s.user, err = auth.CreateUser(s.tlsServer.Auth(), "foo", s.role)
	require.NoError(t, err)

	s.closeContext, s.closeFunc = context.WithCancel(context.Background())

	// Create a in-memory HTTP server that will respond with a UUID. This value
	// will be checked in the client later to ensure a connection was made.
	s.message = uuid.New().String()

	s.testhttp = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, s.message)
	}))
	s.testhttp.Config.TLSConfig = &tls.Config{Time: s.clock.Now}
	s.testhttp.Start()

	// Extract the hostport that the in-memory HTTP server is running on.
	u, err := url.Parse(s.testhttp.URL)
	require.NoError(t, err)
	s.hostport = u.Host

	// Default to staticLabels.
	appLabels := config.AppLabels
	if appLabels == nil {
		appLabels = staticLabels
	}

	// Create apps that will be used for each test.
	appFoo, err := types.NewAppV3(types.Metadata{
		Name:   "foo",
		Labels: appLabels,
	}, types.AppSpecV3{
		URI:           s.testhttp.URL,
		PublicAddr:    "foo.example.com",
		DynamicLabels: types.LabelsToV2(dynamicLabels),
	})
	require.NoError(t, err)
	appAWS, err := types.NewAppV3(types.Metadata{
		Name:   "awsconsole",
		Labels: staticLabels,
	}, types.AppSpecV3{
		URI:        constants.AWSConsoleURL,
		PublicAddr: "aws.example.com",
	})
	require.NoError(t, err)

	// Create a client with a machine role of RoleApp.
	s.authClient, err = s.tlsServer.NewClient(auth.TestServerID(types.RoleApp, s.hostUUID))
	require.NoError(t, err)

	t.Cleanup(func() {
		s.authClient.Close()
	})

	serverIdentity, err := auth.NewServerIdentity(s.authServer.AuthServer, s.hostUUID, types.RoleApp)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)
	tlsConfig.Time = s.clock.Now

	// Generate certificate for user.
	s.clientCertificate = s.generateCertificate(t, s.user, "foo.example.com", "")

	// Generate certificate for AWS console application.
	s.awsConsoleCertificate = s.generateCertificate(t, s.user, "aws.example.com", "readonly")

	lockWatcher, err := services.NewLockWatcher(s.closeContext, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentApp,
			Client:    s.authClient,
		},
	})
	require.NoError(t, err)
	authorizer, err := auth.NewAuthorizer("cluster-name", s.authClient, lockWatcher)
	require.NoError(t, err)

	t.Cleanup(func() {
		lockWatcher.Close()
	})

	apps := types.Apps{appFoo, appAWS}
	if len(config.Apps) > 0 {
		apps = config.Apps
	}

	discard := events.NewDiscardEmitter()

	s.appServer, err = New(s.closeContext, &Config{
		Clock:            s.clock,
		DataDir:          s.dataDir,
		AccessPoint:      s.authClient,
		AuthClient:       s.authClient,
		TLSConfig:        tlsConfig,
		CipherSuites:     utils.DefaultCipherSuites(),
		HostID:           s.hostUUID,
		Hostname:         "test",
		Authorizer:       authorizer,
		GetRotation:      testRotationGetter,
		Apps:             apps,
		OnHeartbeat:      func(err error) {},
		Cloud:            &testCloud{},
		ResourceMatchers: config.ResourceMatchers,
		OnReconcile:      config.OnReconcile,
		LockWatcher:      lockWatcher,
		Emitter:          discard,
		CloudLabels:      config.CloudImporter,
	})
	require.NoError(t, err)

	err = s.appServer.Start(s.closeContext)
	require.NoError(t, err)
	err = s.appServer.ForceHeartbeat()
	require.NoError(t, err)
	t.Cleanup(func() {
		s.appServer.Close()

		// wait for the server to close before allowing other cleanup
		// actions to proceed
		s.appServer.Wait()
	})

	return s
}

func (s *Suite) generateCertificate(t *testing.T, user types.User, publicAddr, AWSRoleARN string) tls.Certificate {
	privateKey, publicKey, err := s.tlsServer.Auth().GenerateKeyPair("")
	require.NoError(t, err)
	req := auth.AppTestCertRequest{
		PublicKey:   publicKey,
		Username:    user.GetName(),
		TTL:         1 * time.Hour,
		PublicAddr:  publicAddr,
		ClusterName: "root.example.com",
	}
	if AWSRoleARN != "" {
		req.AWSRoleARN = AWSRoleARN
	}
	certificate, err := s.tlsServer.Auth().GenerateUserAppTestCert(req)
	require.NoError(t, err)
	tlsCertificate, err := tls.X509KeyPair(certificate, privateKey)
	require.NoError(t, err)

	return tlsCertificate
}

// TestStart makes sure that after the server has started, a correct services.App
// has been created.
func TestStart(t *testing.T) {
	s := SetUpSuite(t)

	// Fetch the services.App that the service heartbeat.
	servers, err := s.authServer.AuthServer.GetApplicationServers(s.closeContext, defaults.Namespace)
	require.NoError(t, err)

	// Check that the services.Server sent via heartbeat is correct. For example,
	// check that the dynamic labels have been evaluated.
	appFoo, err := types.NewAppV3(types.Metadata{
		Name:   "foo",
		Labels: staticLabels,
	}, types.AppSpecV3{
		URI:        s.testhttp.URL,
		PublicAddr: "foo.example.com",
		DynamicLabels: map[string]types.CommandLabelV2{
			dynamicLabelName: {
				Period:  dynamicLabelPeriod,
				Command: dynamicLabelCommand,
				Result:  "4",
			},
		},
	})
	require.NoError(t, err)
	serverFoo, err := types.NewAppServerV3FromApp(appFoo, "test", s.hostUUID)
	require.NoError(t, err)
	appAWS, err := types.NewAppV3(types.Metadata{
		Name:   "awsconsole",
		Labels: staticLabels,
	}, types.AppSpecV3{
		URI:        constants.AWSConsoleURL,
		PublicAddr: "aws.example.com",
	})
	require.NoError(t, err)
	serverAWS, err := types.NewAppServerV3FromApp(appAWS, "test", s.hostUUID)
	require.NoError(t, err)

	sort.Sort(types.AppServers(servers))
	require.Empty(t, cmp.Diff([]types.AppServer{serverAWS, serverFoo}, servers,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Expires")))

	// Check the expiry time is correct.
	for _, server := range servers {
		require.True(t, s.clock.Now().Before(server.Expiry()))
		require.True(t, s.clock.Now().Add(2*defaults.ServerAnnounceTTL).After(server.Expiry()))
	}
}

// TestWaitStop makes sure the server will block and unlock.
func TestWaitStop(t *testing.T) {
	s := SetUpSuite(t)

	// Make sure that wait will block while the server is running.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		s.appServer.Wait()
		cancel()
	}()
	select {
	case <-ctx.Done():
		t.Fatal("Wait failed to block.")
	case <-time.After(250 * time.Millisecond):
	}

	// Close should unblock wait.
	err := s.appServer.Close()
	require.NoError(t, err)
	err = s.appServer.Wait()
	require.Equal(t, err, context.Canceled)
}

func TestAppWithUpdatedLabels(t *testing.T) {
	tests := []struct {
		name          string
		app           types.Application
		dynamicLabels map[string]*labels.Dynamic
		cloudLabels   labels.Importer

		expectedDynamicLabels map[string]string
		expectedCloudLabels   map[string]string
	}{
		{
			name: "no dynamic labels or cloud labels",
			app: &types.AppV3{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"test1": "value1",
					},
				},
			},
			expectedDynamicLabels: map[string]string{},
			expectedCloudLabels:   map[string]string{},
		},
		{
			name: "only dynamic labels",
			app: &types.AppV3{
				Metadata: types.Metadata{
					Name: "app1",
					Labels: map[string]string{
						"test1": "value1",
					},
				},
				Spec: types.AppSpecV3{
					DynamicLabels: map[string]types.CommandLabelV2{
						"something": {
							Command: []string{"echo", "blah"},
							Result:  "blah",
						},
					},
				},
			},
			expectedDynamicLabels: map[string]string{
				"something": "blah",
			},
			expectedCloudLabels: map[string]string{},
		},
		{
			name: "dynamic and cloud labels",
			app: &types.AppV3{
				Metadata: types.Metadata{
					Name: "app1",
					Labels: map[string]string{
						"test1": "value1",
					},
				},
				Spec: types.AppSpecV3{
					DynamicLabels: map[string]types.CommandLabelV2{
						"something": {
							Command: []string{"echo", "blah"},
							Result:  "blah",
						},
					},
				},
			},
			cloudLabels: newTestIMImporter(map[string]string{
				"aws/cloud1": "value1",
				"aws/cloud2": "value2",
			}),
			expectedDynamicLabels: map[string]string{
				"something": "blah",
			},
			expectedCloudLabels: map[string]string{
				"aws/cloud1": "value1",
				"aws/cloud2": "value2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := SetUpSuiteWithConfig(t, suiteConfig{
				CloudImporter: test.cloudLabels,
			})

			if test.cloudLabels != nil {
				require.NoError(t, test.cloudLabels.Sync(context.Background()))
			}

			updatedApp := s.appServer.appWithUpdatedLabels(test.app)

			for key, value := range test.expectedDynamicLabels {
				require.Equal(t, value, updatedApp.GetDynamicLabels()[key].GetResult())
			}

			for key, value := range test.expectedCloudLabels {
				require.Equal(t, value, updatedApp.GetStaticLabels()[key])
			}
		})
	}
}

// testIMImporter is a test instance metadata client for exercising cloud labels.
type testIMImporter struct {
	labels map[string]string
	mu     sync.RWMutex
}

func newTestIMImporter(labels map[string]string) *testIMImporter {
	return &testIMImporter{
		labels: labels,
	}
}

func (i *testIMImporter) Get() map[string]string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	copyLabels := map[string]string{}
	for k, v := range i.labels {
		copyLabels[k] = v
	}
	return copyLabels
}

// Apply adds the current labels to the provided resource's static labels.
func (i *testIMImporter) Apply(r types.ResourceWithLabels) {
	labels := i.Get()
	for k, v := range r.GetStaticLabels() {
		labels[k] = v
	}
	r.SetStaticLabels(labels)
}

func (i *testIMImporter) Sync(context.Context) error {
	return nil
}

func (i *testIMImporter) Start(_ context.Context) {
}

// TestHandleConnection verifies that requests with valid certificates are forwarded and the
// request had headers rewritten as expected.
func TestHandleConnection(t *testing.T) {
	s := SetUpSuite(t)
	s.checkHTTPResponse(t, s.clientCertificate, func(resp *http.Response) {
		require.Equal(t, resp.StatusCode, http.StatusOK)
		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(string(buf)), s.message)
	})
}

// TestAuthorize verifies that only authorized requests are handled.
func TestAuthorize(t *testing.T) {
	tests := []struct {
		name           string
		cloudLabels    labels.Importer
		roleAppLabels  types.Labels
		appLabels      map[string]string
		message        string
		expectedStatus int
	}{
		{
			name: "no cloud labels",
			roleAppLabels: types.Labels{
				"bar": []string{"baz"},
			},
			appLabels: map[string]string{
				"bar": "baz",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "cloud labels",
			cloudLabels: newTestIMImporter(map[string]string{
				"aws/test": "value",
			}),
			roleAppLabels: types.Labels{
				"aws/test": []string{"value"},
			},
			appLabels:      map[string]string{},
			expectedStatus: http.StatusOK,
		},
		{
			name:          "no access",
			roleAppLabels: types.Labels{},
			appLabels: map[string]string{
				"bar": "baz",
			},
			message:        "Not Found",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := SetUpSuiteWithConfig(t, suiteConfig{
				CloudImporter: test.cloudLabels,
				AppLabels:     test.appLabels,
				RoleAppLabels: test.roleAppLabels,
			})

			if test.cloudLabels != nil {
				require.NoError(t, s.appServer.c.CloudLabels.Sync(context.Background()))
			}

			s.checkHTTPResponse(t, s.clientCertificate, func(resp *http.Response) {
				require.Equal(t, test.expectedStatus, resp.StatusCode)
				buf, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				message := s.message
				if test.message != "" {
					message = test.message
				}
				require.Equal(t, message, strings.TrimSpace(string(buf)))
			})
		})
	}
}

// TestAuthorizeWithLocks verifies that requests are forbidden when there is
// a matching lock in force.
func TestAuthorizeWithLocks(t *testing.T) {
	s := SetUpSuite(t)
	// Create a lock targeting the user.
	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{User: s.user.GetName()},
	})
	require.NoError(t, err)
	s.tlsServer.Auth().UpsertLock(s.closeContext, lock)
	defer func() {
		s.tlsServer.Auth().DeleteLock(s.closeContext, lock.GetName())
	}()

	s.checkHTTPResponse(t, s.clientCertificate, func(resp *http.Response) {
		require.Equal(t, resp.StatusCode, http.StatusForbidden)
		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(string(buf)), "Forbidden")
	})
}

// TestGetConfigForClient verifies that only the CAs of the requested cluster are returned.
func TestGetConfigForClient(t *testing.T) {
	// TODO(r0mant): Implement this.
	t.Skip("Not Implemented")
}

// TestRewriteRequest verifies that requests are rewritten to include JWT headers.
func TestRewriteRequest(t *testing.T) {
	// TODO(r0mant): Implement this.
	t.Skip("Not Implemented")
}

// TestRewriteResponse verifies that responses are rewritten if rewrite rules are specified.
func TestRewriteResponse(t *testing.T) {
	// TODO(r0mant): Implement this.
	t.Skip("Not Implemented")
}

// TestSessionClose makes sure sessions are closed after the given session time period.
func TestSessionClose(t *testing.T) {
	// TODO(r0mant): Implement this.
	t.Skip("Not Implemented")
}

// TestAWSConsoleRedirect verifies AWS management console access.
func TestAWSConsoleRedirect(t *testing.T) {
	s := SetUpSuite(t)
	s.checkHTTPResponse(t, s.awsConsoleCertificate, func(resp *http.Response) {
		require.Equal(t, http.StatusFound, resp.StatusCode)
		location, err := resp.Location()
		require.NoError(t, err)
		require.Equal(t, location.String(), "https://signin.aws.amazon.com")
	})
}

// TestRequestAuditEvent verifies that audit events regarding application access
// are being generated with the correct metadata.
// This test configures the server to record the session sync, meaning that the
// events will be forwarded to the auth server. On the server-side, it defines a
// CallbackStreamer, which is going to be used to "intercept" the
// app.session.request events.
func TestRequestAuditEvents(t *testing.T) {
	testhttp := httptest.NewUnstartedServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	testhttp.Config.TLSConfig = &tls.Config{Time: clockwork.NewFakeClock().Now}
	testhttp.Start()

	app, err := types.NewAppV3(types.Metadata{
		Name:   "foo",
		Labels: staticLabels,
	}, types.AppSpecV3{
		URI:           testhttp.URL,
		PublicAddr:    "foo.example.com",
		DynamicLabels: types.LabelsToV2(dynamicLabels),
	})
	require.NoError(t, err)

	requestEventsReceived := atomic.NewUint64(0)
	serverStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
		Inner: events.NewDiscardEmitter(),
		OnEmitAuditEvent: func(_ context.Context, _ libsession.ID, event apievents.AuditEvent) error {
			if event.GetType() == events.AppSessionRequestEvent {
				requestEventsReceived.Inc()

				expectedEvent := &apievents.AppSessionRequest{
					Metadata: apievents.Metadata{
						Type: events.AppSessionRequestEvent,
						Code: events.AppSessionRequestCode,
					},
					AppMetadata: apievents.AppMetadata{
						AppURI:        app.Spec.URI,
						AppPublicAddr: app.Spec.PublicAddr,
						AppName:       app.Metadata.Name,
					},
					StatusCode: 200,
					Method:     "GET",
					Path:       "/",
				}
				require.Empty(t, cmp.Diff(
					expectedEvent,
					event,
					cmpopts.IgnoreTypes(apievents.ServerMetadata{}, apievents.SessionMetadata{}, apievents.UserMetadata{}, apievents.ConnectionMetadata{}),
					cmpopts.IgnoreFields(apievents.Metadata{}, "ID", "ClusterName", "Time"),
					cmpopts.IgnoreFields(apievents.AppSessionChunk{}, "SessionChunkID"),
				))
			}

			return nil
		},
	})
	require.NoError(t, err)

	s := SetUpSuiteWithConfig(t, suiteConfig{
		ServerStreamer: serverStreamer,
		Apps:           types.Apps{app},
	})

	// make a request to generate events.
	s.checkHTTPResponse(t, s.clientCertificate, func(_ *http.Response) {
		// wait until request events are generated before closing the server.
		require.Eventually(t, func() bool {
			return requestEventsReceived.Load() == 1
		}, 500*time.Millisecond, 50*time.Millisecond, "app.request event not generated")
	})

	searchEvents, _, err := s.authServer.AuditLog.SearchEvents(time.Time{}, time.Now().Add(time.Minute), "", []string{events.AppSessionChunkEvent}, 10, types.EventOrderDescending, "")
	require.NoError(t, err)
	require.Len(t, searchEvents, 1)

	expectedEvent := &apievents.AppSessionChunk{
		Metadata: apievents.Metadata{
			Type: events.AppSessionChunkEvent,
			Code: events.AppSessionChunkCode,
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.Spec.URI,
			AppPublicAddr: app.Spec.PublicAddr,
			AppName:       app.Metadata.Name,
		},
	}
	require.Empty(t, cmp.Diff(
		expectedEvent,
		searchEvents[0],
		cmpopts.IgnoreTypes(apievents.ServerMetadata{}, apievents.SessionMetadata{}, apievents.UserMetadata{}, apievents.ConnectionMetadata{}),
		cmpopts.IgnoreFields(apievents.Metadata{}, "ID", "ClusterName"),
		cmpopts.IgnoreFields(apievents.AppSessionChunk{}, "SessionChunkID"),
	))
}

// checkHTTPResponse checks expected HTTP response.
func (s *Suite) checkHTTPResponse(t *testing.T, clientCert tls.Certificate, checkResp func(*http.Response)) {
	pr, pw := net.Pipe()
	defer pw.Close()
	defer pr.Close()

	// Create an HTTP client authenticated with the user's credentials. This acts
	// like the proxy does.
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return pr, nil
			},
			TLSClientConfig: &tls.Config{
				// RootCAs is a pool of host certificates used to verify the identity of
				// the server this client is connecting to.
				RootCAs: s.hostCertPool,
				// Certificates is the user's application specific certificate.
				Certificates: []tls.Certificate{clientCert},
				// Time defines the time anchor for certificate validation
				Time: s.clock.Now,
			},
		},
		// Prevent client from following redirect to be able to test redirect locations.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Handle the connection in another goroutine.
	go func() {
		s.appServer.HandleConnection(pw)
		wg.Done()
	}()

	// Issue request.
	resp, err := httpClient.Get("https://" + constants.APIDomain)
	require.NoError(t, err)

	// Check response.
	checkResp(resp)
	require.NoError(t, resp.Body.Close())

	// Close should not trigger an error.
	require.NoError(t, s.appServer.Close())

	// Wait for the application server to actually stop serving before
	// closing the test. This will make sure the server removes the listeners
	wg.Wait()
}

func testRotationGetter(role types.SystemRole) (*types.Rotation, error) {
	return &types.Rotation{}, nil
}

type testCloud struct{}

func (c *testCloud) GetAWSSigninURL(_ AWSSigninRequest) (*AWSSigninResponse, error) {
	return &AWSSigninResponse{
		SigninURL: "https://signin.aws.amazon.com",
	}, nil
}

var (
	staticLabels = map[string]string{
		"bar": "baz",
	}
	dynamicLabelName    = "qux"
	dynamicLabelPeriod  = types.NewDuration(time.Second)
	dynamicLabelCommand = []string{"expr", "1", "+", "3"}
	dynamicLabels       = map[string]types.CommandLabel{
		dynamicLabelName: &types.CommandLabelV2{
			Period:  dynamicLabelPeriod,
			Command: dynamicLabelCommand,
		},
	}
)

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package alpnproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gravitational/trace"
	"github.com/jackc/pgproto3/v2"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// TestHandleAWSAccessSigVerification tests if LocalProxy verifies the AWS SigV4 signature of incoming request.
func TestHandleAWSAccessSigVerification(t *testing.T) {
	var (
		firstAWSCred  = credentials.NewStaticCredentialsProvider("userID", "firstSecret", "")
		secondAWSCred = credentials.NewStaticCredentialsProvider("userID", "secondSecret", "")
		thirdAWSCred  = credentials.NewStaticCredentialsProvider("userID2", "firstSecret", "")

		awsRegion = "eu-central-1"
	)

	testCases := []struct {
		name       string
		proxyCred  aws.CredentialsProvider
		clientCred aws.CredentialsProvider
		apiOpts    []func(*middleware.Stack) error
		wantStatus int
	}{
		{
			name:       "valid signature",
			proxyCred:  firstAWSCred,
			clientCred: firstAWSCred,
			wantStatus: http.StatusOK,
		},
		{
			name:       "different aws secret access key",
			proxyCred:  secondAWSCred,
			clientCred: firstAWSCred,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "different aws access key ID",
			proxyCred:  thirdAWSCred,
			clientCred: firstAWSCred,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unsigned request",
			proxyCred:  firstAWSCred,
			clientCred: nil,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "signed with User-Agent header",
			proxyCred:  secondAWSCred,
			clientCred: firstAWSCred,
			apiOpts: []func(*middleware.Stack) error{
				func(stack *middleware.Stack) error {
					stack.Finalize.Insert(
						addUserAgentSignedHeaderMiddleware{},
						"Signing",
						middleware.After,
					)
					return nil
				},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lp := createAWSAccessProxySuite(t, tc.proxyCred)

			url := url.URL{
				Scheme: "http",
				Host:   lp.GetAddr(),
				Path:   "/",
			}

			//nolint:forbidigo // OK to not use "stsutils" on tests.
			clt := sts.New(sts.Options{
				APIOptions:       tc.apiOpts,
				Region:           awsRegion,
				Credentials:      tc.clientCred,
				BaseEndpoint:     aws.String(url.String()),
				HTTPClient:       &http.Client{Timeout: 5 * time.Second},
				RetryMaxAttempts: 0,
			})
			_, err := clt.GetCallerIdentity(context.Background(), nil)
			if tc.wantStatus == http.StatusOK {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			var serr *awshttp.ResponseError
			require.ErrorAs(t, err, &serr)
			require.Equal(t, tc.wantStatus, serr.HTTPStatusCode())
		})
	}
}

// Verifies s3 requests are signed without URL escaping to match AWS SDKs.
func TestHandleAWSAccessS3Signing(t *testing.T) {
	provider := credentials.NewStaticCredentialsProvider("access-key", "secret-key", "")
	lp := createAWSAccessProxySuite(t, provider)

	// Avoid loading extra things.
	t.Setenv("AWS_SDK_LOAD_CONFIG", "false")

	// Create a real AWS SDK s3 client.
	s3client := s3.New(s3.Options{
		Region:           "local",
		Credentials:      provider,
		BaseEndpoint:     aws.String("http://" + lp.GetAddr()),
		UsePathStyle:     true,
		HTTPClient:       &http.Client{Timeout: 5 * time.Second},
		RetryMaxAttempts: 0,
	})

	// Use a bucket name with special charaters. AWS SDK actually signs the
	// request with the unescaped bucket name.
	_, err := s3client.ListObjects(context.Background(), &s3.ListObjectsInput{
		Bucket: aws.String("=bucket=name="),
	})

	// Our signature verification should succeed to match what AWS SDK signs.
	require.NoError(t, err)
}

type mockMiddlewareCounter struct {
	sync.Mutex
	recvStateChange chan struct{}
	connCount       int
	startCount      int
}

func newMockMiddlewareCounter() *mockMiddlewareCounter {
	return &mockMiddlewareCounter{
		recvStateChange: make(chan struct{}, 1),
	}
}

func (m *mockMiddlewareCounter) onStateChange() {
	select {
	case m.recvStateChange <- struct{}{}:
	default:
	}
}

func (m *mockMiddlewareCounter) OnNewConnection(_ context.Context, _ *LocalProxy) error {
	m.Lock()
	defer m.Unlock()
	m.connCount++
	m.onStateChange()
	return nil
}

func (m *mockMiddlewareCounter) OnStart(_ context.Context, _ *LocalProxy) error {
	m.Lock()
	defer m.Unlock()
	m.startCount++
	m.onStateChange()
	return nil
}

func (m *mockMiddlewareCounter) waitForCounts(t *testing.T, wantStartCount int, wantConnCount int) {
	timer := time.NewTimer(time.Second * 3)
	for {
		var (
			startCount int
			connCount  int
		)
		m.Lock()
		startCount = m.startCount
		connCount = m.connCount
		m.Unlock()
		if startCount == wantStartCount && connCount == wantConnCount {
			return
		}

		select {
		case <-m.recvStateChange:
			continue
		case <-timer.C:
			require.FailNow(t,
				"timeout waiting for middleware state change",
				"have startCount=%d connCount=%d, want startCount=%d connCount=%d",
				startCount, connCount, wantStartCount, wantConnCount)
		}
	}
}

var _ LocalProxyMiddleware = (*mockMiddlewareCounter)(nil)

func TestMiddleware(t *testing.T) {
	m := newMockMiddlewareCounter()
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	lp, err := NewLocalProxy(LocalProxyConfig{
		Listener:           mustCreateLocalListener(t),
		RemoteProxyAddr:    hs.Listener.Addr().String(),
		Protocols:          []common.Protocol{common.ProtocolHTTP},
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
		Middleware:         m,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := lp.Close()
		require.NoError(t, err)
		hs.Close()
	})

	m.waitForCounts(t, 0, 0)
	go func() {
		err := lp.Start(context.Background())
		require.NoError(t, err)
	}()

	// ensure that OnStart middleware is called when the proxy starts
	m.waitForCounts(t, 1, 0)
	url := url.URL{
		Scheme: "http",
		Host:   lp.GetAddr(),
		Path:   "/",
	}

	pr := bytes.NewReader([]byte("payload content"))
	req, err := http.NewRequest(http.MethodGet, url.String(), pr)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	// ensure that OnNewConnection middleware is called when a new connection is made to the proxy
	m.waitForCounts(t, 1, 1)
}

// mockCertRenewer is a mock middleware for the local proxy that always sets the local proxy certs slice.
type mockCertRenewer struct {
	cert tls.Certificate
}

func (m *mockCertRenewer) OnNewConnection(_ context.Context, lp *LocalProxy) error {
	lp.SetCert(m.cert)
	return nil
}

func (m *mockCertRenewer) OnStart(_ context.Context, lp *LocalProxy) error {
	lp.SetCert(m.cert)
	return nil
}

// TestLocalProxyConcurrentCertRenewal tests for data races in local proxy cert renewal.
func TestLocalProxyConcurrentCertRenewal(t *testing.T) {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})
	lp, err := NewLocalProxy(LocalProxyConfig{
		Listener:           mustCreateLocalListener(t),
		RemoteProxyAddr:    hs.Listener.Addr().String(),
		Protocols:          []common.Protocol{common.ProtocolHTTP},
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
		Middleware:         &mockCertRenewer{tls.Certificate{}},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		lp.Close()
	})
	go func() {
		assert.NoError(t, lp.Start(context.Background()))
	}()

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", lp.GetAddr())
			assert.NoError(t, err)
			assert.NoError(t, conn.Close())
		}()
	}
	wg.Wait()
}

func TestCheckDBCerts(t *testing.T) {
	suite := NewSuite(t)
	dbRouteInCert := tlsca.RouteToDatabase{
		ServiceName: "svc1",
		Protocol:    defaults.ProtocolPostgres,
		Username:    "user1",
		Database:    "db1",
	}
	clockValid := clockwork.NewFakeClockAt(time.Now())
	clockAfterValid := clockwork.NewFakeClockAt(clockValid.Now().Add(time.Hour))
	clockBeforeValid := clockwork.NewFakeClockAt(clockValid.Now().Add(-time.Hour))

	// we wont actually be listening for connections, but local proxy config needs to be valid to pass checks.
	lp, err := NewLocalProxy(LocalProxyConfig{
		RemoteProxyAddr:    "localhost",
		Protocols:          []common.Protocol{common.ProtocolPostgres},
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
		// freeze the local proxy in time, we'll be manipulating fakeClock for cert generation.
		Clock: clockwork.NewFakeClockAt(clockValid.Now()),
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		clock       clockwork.Clock
		dbRoute     tlsca.RouteToDatabase
		errAssertFn require.ErrorAssertionFunc
	}{
		{
			name:        "detects clock is after cert expires",
			clock:       clockAfterValid,
			dbRoute:     dbRouteInCert,
			errAssertFn: requireExpiredCertErr,
		},
		{
			name:        "detects clock is before cert is valid",
			clock:       clockBeforeValid,
			dbRoute:     dbRouteInCert,
			errAssertFn: requireExpiredCertErr,
		},
		{
			name:  "detects that cert subject user does not match db route",
			clock: clockValid,
			dbRoute: tlsca.RouteToDatabase{
				ServiceName: "svc1",
				Protocol:    defaults.ProtocolPostgres,
				Username:    "user2",
				Database:    "db1",
			},
			errAssertFn: requireCertSubjectUserErr,
		},
		{
			name:  "detects that cert subject database does not match db route",
			clock: clockValid,
			dbRoute: tlsca.RouteToDatabase{
				ServiceName: "svc1",
				Protocol:    defaults.ProtocolPostgres,
				Username:    "user1",
				Database:    "db2",
			},
			errAssertFn: requireCertSubjectDatabaseErr,
		},
		{
			name:        "valid cert",
			clock:       clockValid,
			dbRoute:     dbRouteInCert,
			errAssertFn: require.NoError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tlsCert := mustGenCertSignedWithCA(t, suite.ca,
				withIdentity(tlsca.Identity{
					Username:        "test-user",
					Groups:          []string{"test-group"},
					RouteToDatabase: dbRouteInCert,
				}),
				withClock(tt.clock),
			)
			lp.SetCert(tlsCert)
			tt.errAssertFn(t, lp.CheckDBCert(context.Background(), tt.dbRoute))
		})
	}
}

type mockMiddlewareConnUnauth struct {
}

func (m *mockMiddlewareConnUnauth) OnNewConnection(_ context.Context, _ *LocalProxy) error {
	return trace.AccessDenied("access denied.")
}

func (m *mockMiddlewareConnUnauth) OnStart(_ context.Context, _ *LocalProxy) error {
	return nil
}

var _ LocalProxyMiddleware = (*mockMiddlewareConnUnauth)(nil)

func TestLocalProxyClosesConnOnError(t *testing.T) {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	lp, err := NewLocalProxy(LocalProxyConfig{
		Listener:           mustCreateLocalListener(t),
		RemoteProxyAddr:    hs.Listener.Addr().String(),
		Protocols:          []common.Protocol{common.ProtocolHTTP},
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
		Middleware:         &mockMiddlewareConnUnauth{},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := lp.Close()
		require.NoError(t, err)
		hs.Close()
	})
	go func() {
		assert.NoError(t, lp.Start(context.Background()))
	}()

	conn, err := net.Dial("tcp", lp.GetAddr())
	require.NoError(t, err)

	// set a read deadline so that if the connection is not closed,
	// this test will fail quickly instead of hanging.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 512)
	_, err = conn.Read(buf)
	require.Error(t, err)
	require.ErrorIs(t, err, io.EOF, "connection should have been closed by local proxy")
}

func TestKubeMiddleware(t *testing.T) {
	t.Parallel()

	now := time.Now()
	clock := clockwork.NewFakeClockAt(now)
	teleportCluster := "localhost"

	ca := mustGenSelfSignedCert(t)
	kube1Cert := mustGenCertSignedWithCA(t, ca,
		withIdentity(tlsca.Identity{
			Username:          "test-user",
			Groups:            []string{"test-group"},
			KubernetesCluster: "kube1",
		}),
		withClock(clock),
	)
	kube2Cert := mustGenCertSignedWithCA(t, ca,
		withIdentity(tlsca.Identity{
			Username:          "test-user",
			Groups:            []string{"test-group"},
			KubernetesCluster: "kube2",
		}),
		withClock(clock),
	)
	newCert := mustGenCertSignedWithCA(t, ca,
		withIdentity(tlsca.Identity{
			Username:          "test-user",
			Groups:            []string{"test-group"},
			KubernetesCluster: "kube1newCert",
		}),
		withClock(clock),
	)

	certReissuer := func(ctx context.Context, teleportCluster, kubeCluster string) (tls.Certificate, error) {
		select {
		case <-ctx.Done():
			return tls.Certificate{}, ctx.Err()
		default:
			return newCert, nil
		}
	}

	t.Run("expired certificate is still reissued if request context expires", func(t *testing.T) {
		req := &http.Request{
			TLS: &tls.ConnectionState{
				ServerName: common.KubeLocalProxySNI(teleportCluster, "kube1"),
			},
		}
		// we set request context to a context that is already canceled, so handler function will start reissuing
		// certificate goroutine and then will exit immediately.
		reqCtx, cancel := context.WithCancel(context.Background())
		cancel()
		req = req.WithContext(reqCtx)

		startCerts := KubeClientCerts{}
		startCerts.Add(teleportCluster, "kube1", kube1Cert)
		km := NewKubeMiddleware(KubeMiddlewareConfig{
			Certs:        startCerts,
			CertReissuer: certReissuer,
			Clock:        clockwork.NewFakeClockAt(now.Add(time.Hour * 2)),
			CloseContext: context.Background(),
		})
		err := km.CheckAndSetDefaults()
		require.NoError(t, err)

		var rw *responsewriters.MemoryResponseWriter
		// We use `require.Eventually` to avoid a very rare test flakiness case when reissue goroutine manages to
		// successfully finish before the parent goroutine has a chance to check the context (and see that it's expired).
		require.Eventually(t, func() bool {
			rw = responsewriters.NewMemoryResponseWriter()
			// HandleRequest will reissue certificate if needed.
			km.HandleRequest(rw, req)

			// request timed out.
			return rw.Status() == http.StatusInternalServerError

		}, 5*time.Second, 100*time.Millisecond)
		require.Contains(t, rw.Buffer().String(), "context canceled")

		// just let the reissuing goroutine some time to replace certs.
		time.Sleep(10 * time.Millisecond)

		// but certificate still was reissued.
		certs, err := km.OverwriteClientCerts(req)
		require.NoError(t, err)
		require.Len(t, certs, 1)
		require.Equal(t, newCert, certs[0], "certificate was not reissued")
	})

	getStartCerts := func() KubeClientCerts {
		certs := KubeClientCerts{}
		certs.Add(teleportCluster, "kube1", kube1Cert)
		certs.Add(teleportCluster, "kube2", kube2Cert)
		return certs
	}
	testCases := []struct {
		name            string
		reqClusterName  string
		startCerts      KubeClientCerts
		clock           clockwork.Clock
		overwrittenCert tls.Certificate
		wantErr         string
	}{
		{
			name:            "reissue cert when not found",
			reqClusterName:  "kube3",
			startCerts:      getStartCerts(),
			clock:           clockwork.NewFakeClockAt(now),
			overwrittenCert: newCert,
			wantErr:         "",
		},
		{
			name:            "expired cert is reissued",
			reqClusterName:  "kube1",
			startCerts:      getStartCerts(),
			clock:           clockwork.NewFakeClockAt(now.Add(time.Hour * 2)),
			overwrittenCert: newCert,
			wantErr:         "",
		},
		{
			name:            "valid cert for kube1 is returned",
			reqClusterName:  "kube1",
			startCerts:      getStartCerts(),
			clock:           clockwork.NewFakeClockAt(now),
			overwrittenCert: kube1Cert,
			wantErr:         "",
		},
		{
			name:            "valid cert for kube2 is returned",
			reqClusterName:  "kube2",
			startCerts:      getStartCerts(),
			clock:           clockwork.NewFakeClockAt(now),
			overwrittenCert: kube2Cert,
			wantErr:         "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := http.Request{
				TLS: &tls.ConnectionState{
					ServerName: common.KubeLocalProxySNI(teleportCluster, tt.reqClusterName),
				},
			}
			km := NewKubeMiddleware(KubeMiddlewareConfig{
				Certs:        tt.startCerts,
				CertReissuer: certReissuer,
				Logger:       utils.NewSlogLoggerForTests(),
				Clock:        tt.clock,
				CloseContext: context.Background(),
			})

			// HandleRequest will reissue certificate if needed
			km.HandleRequest(responsewriters.NewMemoryResponseWriter(), &req)

			certs, err := km.OverwriteClientCerts(&req)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Len(t, certs, 1)
				require.Equal(t, tt.overwrittenCert, certs[0])
			}
		})
	}
}

func createAWSAccessProxySuite(t *testing.T, provider aws.CredentialsProvider) *LocalProxy {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))

	lp, err := NewLocalProxy(LocalProxyConfig{
		Listener:           mustCreateLocalListener(t),
		RemoteProxyAddr:    hs.Listener.Addr().String(),
		Protocols:          []common.Protocol{common.ProtocolHTTP},
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
		HTTPMiddleware:     &AWSAccessMiddleware{AWSCredentialsProvider: provider},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := lp.Close()
		require.NoError(t, err)
		hs.Close()
	})
	go func() {
		err := lp.Start(context.Background())
		assert.NoError(t, err)
	}()
	return lp
}

func requireExpiredCertErr(t require.TestingT, err error, _ ...interface{}) {
	if h, ok := t.(*testing.T); ok {
		h.Helper()
	}
	require.Error(t, err)
	var certErr x509.CertificateInvalidError
	require.ErrorAs(t, err, &certErr)
	require.Equal(t, x509.Expired, certErr.Reason)
}

func requireCertSubjectUserErr(t require.TestingT, err error, _ ...interface{}) {
	if h, ok := t.(*testing.T); ok {
		h.Helper()
	}
	require.Error(t, err)
	require.ErrorContains(t, err, "certificate subject is for user")
}

func requireCertSubjectDatabaseErr(t require.TestingT, err error, _ ...interface{}) {
	if h, ok := t.(*testing.T); ok {
		h.Helper()
	}
	require.Error(t, err)
	require.ErrorContains(t, err, "certificate subject is for database name")
}

// stubConn implements net.Conn interface and is used to stub a client
// connection to local proxy.
type stubConn struct {
	net.Conn
	buff bytes.Buffer
}

func (c *stubConn) Write(p []byte) (n int, err error) {
	return c.buff.Write(p)
}
func (c *stubConn) Read(p []byte) (n int, err error) {
	return c.buff.Read(p)
}

func TestGetCertsForConn(t *testing.T) {
	suite := NewSuite(t)
	dbRouteInCert := tlsca.RouteToDatabase{
		ServiceName: "svc1",
		Protocol:    defaults.ProtocolPostgres,
		Username:    "user1",
		Database:    "db1",
	}
	tlsCert := mustGenCertSignedWithCA(t, suite.ca,
		withIdentity(tlsca.Identity{
			Username:        "test-user",
			Groups:          []string{"test-group"},
			RouteToDatabase: dbRouteInCert,
		}),
	)

	tests := map[string]struct {
		checkCertsNeeded bool
		addProtocols     []common.Protocol
		stubConnBytes    []byte
		wantCerts        bool
	}{
		"tunnel always": {
			checkCertsNeeded: false,
			wantCerts:        true,
		},
		"no tunnel when not needed for protocol": {
			checkCertsNeeded: true,
			wantCerts:        false,
		},
		"no tunnel when not needed for postgres protocol": {
			checkCertsNeeded: true,
			addProtocols:     []common.Protocol{common.ProtocolPostgres},
			stubConnBytes: func() []byte {
				val, err := (&pgproto3.SSLRequest{}).Encode(nil)
				require.NoError(t, err, "SSLRequest.Encode failed")
				return val
			}(),
			wantCerts: false,
		},
		"tunnel when needed for postgres protocol": {
			checkCertsNeeded: true,
			addProtocols:     []common.Protocol{common.ProtocolPostgres},
			stubConnBytes: func() []byte {
				val, err := (&pgproto3.CancelRequest{}).Encode(nil)
				require.NoError(t, err, "CancelRequest.Encode failed")
				return val
			}(),
			wantCerts: true,
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// we wont actually be listening for connections, but local proxy config needs to be valid to pass checks.
			lp, err := NewLocalProxy(LocalProxyConfig{
				RemoteProxyAddr: "localhost",
				Protocols:       append([]common.Protocol{"foo-bar-proto"}, tt.addProtocols...),
				ParentContext:   context.Background(),
				CheckCertNeeded: tt.checkCertsNeeded,
				Cert:            tlsCert,
			})
			require.NoError(t, err)
			conn := &stubConn{buff: *bytes.NewBuffer(tt.stubConnBytes)}
			gotCert, _, err := lp.getCertForConn(conn)
			require.NoError(t, err)
			if tt.wantCerts {
				require.Equal(t, tlsCert, gotCert)
			} else {
				require.Empty(t, gotCert)
			}
		})
	}
}

type addUserAgentSignedHeaderMiddleware struct {
}

func (m addUserAgentSignedHeaderMiddleware) ID() string { return "AddUserAgentSignedHeader" }
func (m addUserAgentSignedHeaderMiddleware) HandleFinalize(
	ctx context.Context,
	in middleware.FinalizeInput,
	next middleware.FinalizeHandler,
) (out middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, trace.Errorf("unexpected request middleware type %T", in.Request)
	}

	authHeader := req.Header.Get("Authorization")
	authHeader = strings.Replace(authHeader, "SignedHeaders=", "SignedHeaders=user-agent;", 1)
	req.Header.Set("Authorization", authHeader)
	return next.HandleFinalize(ctx, in)
}

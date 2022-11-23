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
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestHandleAWSAccessSigVerification tests if LocalProxy verifies the AWS SigV4 signature of incoming request.
func TestHandleAWSAccessSigVerification(t *testing.T) {
	var (
		firstAWSCred  = credentials.NewStaticCredentials("userID", "firstSecret", "")
		secondAWSCred = credentials.NewStaticCredentials("userID", "secondSecret", "")
		thirdAWSCred  = credentials.NewStaticCredentials("userID2", "firstSecret", "")

		awsService = "s3"
		awsRegion  = "eu-central-1"
	)

	testCases := []struct {
		name       string
		originCred *credentials.Credentials
		proxyCred  *credentials.Credentials
		wantErr    require.ErrorAssertionFunc
		wantStatus int
	}{
		{
			name:       "valid signature",
			originCred: firstAWSCred,
			proxyCred:  firstAWSCred,
			wantErr:    require.NoError,
			wantStatus: http.StatusOK,
		},
		{
			name:       "different aws secret access key",
			originCred: firstAWSCred,
			proxyCred:  secondAWSCred,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "different aws access key ID",
			originCred: firstAWSCred,
			proxyCred:  thirdAWSCred,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unsigned request",
			originCred: nil,
			proxyCred:  firstAWSCred,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lp := createAWSAccessProxySuite(t, tc.proxyCred)

			url := url.URL{
				Scheme: "http",
				Host:   lp.GetAddr(),
				Path:   "/",
			}

			pr := bytes.NewReader([]byte("payload content"))
			req, err := http.NewRequest(http.MethodGet, url.String(), pr)
			require.NoError(t, err)

			if tc.originCred != nil {
				v4.NewSigner(tc.originCred).Sign(req, pr, awsService, awsRegion, time.Now())
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
	}
}

// Verifies s3 requests are signed without URL escaping to match AWS SDKs.
func TestHandleAWSAccessS3Signing(t *testing.T) {
	cred := credentials.NewStaticCredentials("access-key", "secret-key", "")
	lp := createAWSAccessProxySuite(t, cred)

	// Avoid loading extra things.
	t.Setenv("AWS_SDK_LOAD_CONFIG", "false")

	// Create a real AWS SDK s3 client.
	awsConfig := aws.NewConfig().
		WithDisableSSL(true).
		WithRegion("local").
		WithCredentials(cred).
		WithEndpoint(lp.GetAddr()).
		WithS3ForcePathStyle(true)

	s3client := s3.New(session.Must(session.NewSession(awsConfig)))

	// Use a bucket name with special charaters. AWS SDK actually signs the
	// request with the unescaped bucket name.
	_, err := s3client.ListObjects(&s3.ListObjectsInput{
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

func (m *mockMiddlewareCounter) OnNewConnection(_ context.Context, _ *LocalProxy, _ net.Conn) error {
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
type mockCertRenewer struct{}

func (m *mockCertRenewer) OnNewConnection(_ context.Context, lp *LocalProxy, _ net.Conn) error {
	lp.SetCerts([]tls.Certificate{})
	return nil
}

func (m *mockCertRenewer) OnStart(_ context.Context, lp *LocalProxy) error {
	lp.SetCerts([]tls.Certificate{})
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
		Middleware:         &mockCertRenewer{},
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
			lp.SetCerts([]tls.Certificate{tlsCert})
			tt.errAssertFn(t, lp.CheckDBCerts(tt.dbRoute))
		})
	}
}

type mockMiddlewareConnUnauth struct {
}

func (m *mockMiddlewareConnUnauth) OnNewConnection(_ context.Context, _ *LocalProxy, _ net.Conn) error {
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
	require.ErrorIs(t, err, io.EOF)
}

func createAWSAccessProxySuite(t *testing.T, cred *credentials.Credentials) *LocalProxy {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))

	lp, err := NewLocalProxy(LocalProxyConfig{
		Listener:           mustCreateLocalListener(t),
		RemoteProxyAddr:    hs.Listener.Addr().String(),
		Protocols:          []common.Protocol{common.ProtocolHTTP},
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
		AWSCredentials:     cred,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := lp.Close()
		require.NoError(t, err)
		hs.Close()
	})
	go func() {
		err := lp.StartAWSAccessProxy(context.Background())
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
	require.Equal(t, certErr.Reason, x509.Expired)
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

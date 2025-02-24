/*
Copyright 2015 The Kubernetes Authors.

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

package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/httpstream"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/third_party/forked/golang/netutil"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth"
)

// SpdyRoundTripper knows how to upgrade an HTTP request to one that supports
// multiplexed streams. After RoundTrip() is invoked, Conn will be set
// and usable. SpdyRoundTripper implements the UpgradeRoundTripper interface.
type SpdyRoundTripper struct {
	roundTripperConfig

	/* TODO according to http://golang.org/pkg/net/http/#RoundTripper, a RoundTripper
	   must be safe for use by multiple concurrent goroutines. If this is absolutely
	   necessary, we could keep a map from http.Request to net.Conn. In practice,
	   a client will create an http.Client, set the transport to a new insteace of
	   SpdyRoundTripper, and use it a single time, so this hopefully won't be an issue.
	*/
	// conn is the underlying network connection to the remote server.
	conn net.Conn
}

var (
	_ utilnet.TLSClientConfigHolder  = &SpdyRoundTripper{}
	_ httpstream.UpgradeRoundTripper = &SpdyRoundTripper{}
	_ utilnet.Dialer                 = &SpdyRoundTripper{}
)

type roundTripperConfig struct {
	// ctx is a context for this round tripper
	ctx context.Context
	// sess is the cluster session
	sess *clusterSession
	// dialWithContext is the function used connect to remote address
	dialWithContext dialContextFunc
	// tlsConfig holds the TLS configuration settings to use when connecting
	// to the remote server.
	tlsConfig *tls.Config
	// pingPeriod is the period at which to send pings to the remote server to
	// keep the SPDY connection alive.
	pingPeriod time.Duration
	// originalHeaders are the headers that were passed from the original request.
	// These headers are used to set the headers on the new request if the user
	// requested Kubernetes impersonation.
	originalHeaders http.Header
	// useIdentityForwarding controls whether the proxy should forward the
	// identity of the user making the request to the remote server using the
	// auth.TeleportImpersonateUserHeader and auth.TeleportImpersonateIPHeader
	// headers instead of relying on the certificate to transport it.
	useIdentityForwarding bool
	// log specifies the logger.
	log *slog.Logger

	proxier func(*http.Request) (*url.URL, error)
}

// NewSpdyRoundTripperWithDialer creates a new SpdyRoundTripper that will use
// the specified tlsConfig. This function is mostly meant for unit tests.
func NewSpdyRoundTripperWithDialer(cfg roundTripperConfig) *SpdyRoundTripper {
	return &SpdyRoundTripper{roundTripperConfig: cfg}
}

// TLSClientConfig implements pkg/util/net.TLSClientConfigHolder for proper TLS checking during
// proxying with a spdy roundtripper.
func (s *SpdyRoundTripper) TLSClientConfig() *tls.Config {
	return s.tlsConfig
}

// Dial implements k8s.io/apimachinery/pkg/util/net.Dialer.
func (s *SpdyRoundTripper) Dial(req *http.Request) (net.Conn, error) {
	conn, err := s.dial(req)
	if err != nil {
		return nil, err
	}
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// dial dials the host specified by url, using TLS if appropriate.
func (s *SpdyRoundTripper) dial(req *http.Request) (conn net.Conn, err error) {
	var proxyURL *url.URL
	if s.proxier != nil {
		proxyURL, err = s.proxier(req)
		if err != nil {
			return nil, err
		}
	}

	if proxyURL == nil {
		conn, err = s.dialWithoutProxy(req.URL)
	} else {
		conn, err = s.dialWithProxy(req, proxyURL)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.URL.Scheme == "https" {
		return s.tlsConn(s.ctx, conn, netutil.CanonicalAddr(req.URL))
	}
	return conn, nil
}

func (s *SpdyRoundTripper) dialWithoutProxy(url *url.URL) (conn net.Conn, err error) {
	dialAddr := netutil.CanonicalAddr(url)
	switch {
	case s.dialWithContext != nil:
		conn, err = s.dialWithContext(s.ctx, "tcp", dialAddr)
	default:
		conn, err = net.Dial("tcp", dialAddr)
	}
	return conn, trace.Wrap(err)
}

// tlsConn returns a TLS client side connection using rwc as the underlying transport.
func (s *SpdyRoundTripper) tlsConn(ctx context.Context, rwc net.Conn, targetHost string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(targetHost)
	if err != nil {
		return nil, err
	}

	tlsConfig := s.tlsConfig
	switch {
	case tlsConfig == nil:
		tlsConfig = &tls.Config{ServerName: host}
	case len(tlsConfig.ServerName) == 0:
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = host
	}
	tlsConn := tls.Client(rwc, tlsConfig)

	// Client handshake will verify the server hostname and cert chain. That
	// way we can err our before first read/write.
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		tlsConn.Close()
		return nil, trace.Wrap(err)
	}

	return tlsConn, nil
}

// dialWithProxy dials the host specified by url through an http or an socks5 proxy.
func (s *SpdyRoundTripper) dialWithProxy(req *http.Request, proxyURL *url.URL) (net.Conn, error) {
	// ensure we use a canonical host with proxyReq
	targetHost := netutil.CanonicalAddr(req.URL)

	proxyDialConn, err := apiclient.DialProxyWithDialer(
		s.ctx,
		proxyURL,
		targetHost,
		apiclient.ContextDialerFunc(s.dialWithContext),
	)
	return proxyDialConn, trace.Wrap(err)
}

// RoundTrip executes the Request and upgrades it. After a successful upgrade,
// clients may call SpdyRoundTripper.Connection() to retrieve the upgraded
// connection.
func (s *SpdyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	header := utilnet.CloneHeader(req.Header)
	// copyImpersonationHeaders copies the headers from the original request to the new
	// request headers. This is necessary to forward the original user's impersonation
	// when multiple kubernetes_users are available.
	copyImpersonationHeaders(header, s.originalHeaders)
	header.Set(httpstream.HeaderConnection, httpstream.HeaderUpgrade)
	header.Set(httpstream.HeaderUpgrade, streamspdy.HeaderSpdy31)
	if err := setupImpersonationHeaders(s.sess, header); err != nil {
		return nil, trace.Wrap(err)
	}

	var (
		conn        net.Conn
		rawResponse []byte
		err         error
	)

	// If we're using identity forwarding, we need to add the impersonation
	// headers to the request before we send the request.
	if s.useIdentityForwarding {
		if header, err = auth.IdentityForwardingHeaders(s.ctx, header); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	clone := utilnet.CloneRequest(req)
	clone.Header = header
	conn, err = s.Dial(clone)
	if err != nil {
		return nil, err
	}

	responseReader := bufio.NewReader(
		io.MultiReader(
			bytes.NewBuffer(rawResponse),
			conn,
		),
	)
	resp, err := http.ReadResponse(responseReader, nil)
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		return nil, err
	}

	s.conn = conn

	return resp, nil
}

// NewConnection validates the upgrade response, creating and returning a new
// httpstream.Connection if there were no errors.
func (s *SpdyRoundTripper) NewConnection(resp *http.Response) (httpstream.Connection, error) {
	connectionHeader := strings.ToLower(resp.Header.Get(httpstream.HeaderConnection))
	upgradeHeader := strings.ToLower(resp.Header.Get(httpstream.HeaderUpgrade))
	if (resp.StatusCode != http.StatusSwitchingProtocols) || !strings.Contains(connectionHeader, strings.ToLower(httpstream.HeaderUpgrade)) || !strings.Contains(upgradeHeader, strings.ToLower(streamspdy.HeaderSpdy31)) {
		return nil, trace.Wrap(extractKubeAPIStatusFromReq(resp))
	}

	return streamspdy.NewClientConnectionWithPings(s.conn, s.pingPeriod)
}

// statusScheme is private scheme for the decoding here until someone fixes the TODO in NewConnection
var statusScheme = runtime.NewScheme()

// ParameterCodec knows about query parameters used with the meta v1 API spec.
var statusCodecs = serializer.NewCodecFactory(statusScheme)

func init() {
	statusScheme.AddUnversionedTypes(metav1.SchemeGroupVersion,
		&metav1.Status{},
	)
}

// extractKubeAPIStatusFromReq extracts the status from the response body and returns it as an error.
func extractKubeAPIStatusFromReq(rsp *http.Response) error {
	defer func() {
		_ = rsp.Body.Close()
	}()
	responseError := ""
	responseErrorBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		responseError = "unable to read error from server response"
	} else {
		if obj, _, err := statusCodecs.UniversalDecoder().Decode(responseErrorBytes, nil, &metav1.Status{}); err == nil {
			if status, ok := obj.(*metav1.Status); ok {
				return &upgradeFailureError{Cause: &apierrors.StatusError{ErrStatus: *status}}
			}
		}
		responseError = string(responseErrorBytes)
		responseError = strings.TrimSpace(responseError)
	}
	return &upgradeFailureError{Cause: fmt.Errorf("unable to upgrade connection: %s", responseError)}
}

// upgradeFailureError encapsulates the cause for why the streaming
// upgrade request failed. Implements error interface.
type upgradeFailureError struct {
	Cause error
}

func (u *upgradeFailureError) Error() string {
	return u.Cause.Error()
}

func (u *upgradeFailureError) Unwrap() error {
	return u.Cause
}

func isTeleportUpgradeFailure(err error) bool {
	var upgradeErr *upgradeFailureError
	return errors.As(err, &upgradeErr)
}

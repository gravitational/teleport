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
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

var alpnTestResultsCache = utils.NewCache[string, bool](utils.CacheConfig{
	DefaultTTL: time.Hour,
})

// isHTTPConnUpgradeRequired returns true if a tunnel is required through a HTTP
// upgrade.
//
// The function makes a test connection to the Proxy Service and checks if the
// Teleport custom ALPN protocols are preserved. If not, the Proxy Service is
// likely behind an AWS ALB or some custom proxy services that strip out ALPN
// and SNI information on the way to our Proxy Service.
//
// In those cases, the client makes a HTTP "upgrade" call to the Proxy Service
// to establish a tunnel for the origianlly planned traffic.
func isHTTPConnUpgradeRequired(proxyAddr string, tlsConfig *tls.Config) bool {
	// TODO Currently if remote is not HTTPS, TLS routing is not performed at
	// all. However, the HTTP upgrade is probably a good workaround for HTTP
	// remotes.
	if tlsConfig == nil {
		return false
	}

	if result, found := alpnTestResultsCache.Get(proxyAddr); found {
		return result
	}

	upgradeRequired, err := alpnHandshakeTest(
		proxyAddr,
		// Use an old but stable protocol for testing to reduce false
		// positives in case remote is running a different version.
		constants.ALPNSNIProtocolReverseTunnel,
		tlsConfig.InsecureSkipVerify,
	)

	// Do NOT cache when it fails.
	if err == nil {
		alpnTestResultsCache.Set(proxyAddr, upgradeRequired)
	}

	return upgradeRequired
}

func alpnHandshakeTest(addr string, protocol string, insecure bool) (bool, error) {
	logrus.Debugf("-->> alpnHandshakeTest test for %v", addr)

	if utils.IsLoopback(addr) || utils.IsUnspecified(addr) {
		return false, nil
	}

	testConn, err := tls.Dial("tcp", addr, &tls.Config{
		NextProtos:         []string{protocol},
		InsecureSkipVerify: insecure,
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	defer testConn.Close()
	return testConn.ConnectionState().NegotiatedProtocol == "", nil
}

// TODO
type httpConnUpgradeDialer struct {
	insecure bool
}

func newHTTPConnUpgradeDialer(insecure bool) ContextDialer {
	return &httpConnUpgradeDialer{
		insecure: insecure,
	}
}

func (d httpConnUpgradeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		InsecureSkipVerify: d.insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)

	}

	url := url.URL{
		Host:   addr,
		Scheme: "https",
		Path:   constants.ConnectionUpgradeWebAPI,
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// For now, only ALPN is supported.
	req.Header.Add(constants.ConnectionUpgradeHeader, constants.ConnectionUpgradeTypeALPN)

	if err = req.Write(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if http.StatusSwitchingProtocols != resp.StatusCode {
		if http.StatusNotFound == resp.StatusCode {
			return nil, trace.NotImplemented(
				"connection upgrade failed with status code %v. please upgrade the server and try again.",
				resp.StatusCode,
			)
		}
		return nil, trace.BadParameter("failed to switch Protocols %v", resp.StatusCode)
	}

	// TODO add ping conn
	return conn, nil
}

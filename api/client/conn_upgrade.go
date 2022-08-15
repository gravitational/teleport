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

// IsALPNConnUpgradeRequired returns true if a tunnel is required through a HTTP
// connection upgrade.
//
// The function makes a test connection to the Proxy Service and checks if the
// Teleport custom ALPN protocols are preserved. If not, the Proxy Service is
// likely behind an AWS ALB or some custom proxy services that strip out ALPN
// and SNI information on the way to our Proxy Service.
//
// In those cases, the client makes a HTTP "upgrade" call to the Proxy Service
// to establish a tunnel for the origianlly planned traffic to preserve the
// ALPN and SNI information.
func IsALPNConnUpgradeRequired(addr string, insecure bool) bool {
	if utils.IsLoopback(addr) || utils.IsUnspecified(addr) {
		logrus.Debugf("ALPN connection upgrade not required because %q is either unspecified or a loopback.", addr)
		return false
	}

	// Use an old but stable protocol for testing to reduce false
	// positives in case remote is running a different version.
	testProtocol := constants.ALPNSNIProtocolReverseTunnel
	testConn, err := tls.Dial("tcp", addr, &tls.Config{
		NextProtos:         []string{testProtocol},
		InsecureSkipVerify: insecure,
	})
	if err != nil {
		logrus.Infof("ALPN connection upgrade test failed for %q: %v.", addr, err)
		return false
	}
	defer testConn.Close()

	result := testConn.ConnectionState().NegotiatedProtocol == ""
	logrus.Debugf("ALPN connection upgrade is required for %q: %v.", addr, result)
	return result
}

// alpnConnUpgradeDialer makes an "HTTP" upgrade call to the Proxy Service then
// tunnels the connection with this connection upgrade.
type alpnConnUpgradeDialer struct {
	netDialer *net.Dialer
	insecure  bool
}

// newALPNConnUpgradeDialer creates a new alpnConnUpgradeDialer.
func newALPNConnUpgradeDialer(keepAlivePeriod, dialTimeout time.Duration, insecure bool) ContextDialer {
	return &alpnConnUpgradeDialer{
		insecure: insecure,
		netDialer: &net.Dialer{
			KeepAlive: keepAlivePeriod,
			Timeout:   dialTimeout,
		},
	}
}

// DialContext implements ContextDialer
func (d alpnConnUpgradeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Make a TLS connection for the https call.
	tlsConn, err := tls.DialWithDialer(d.netDialer, network, addr, &tls.Config{
		InsecureSkipVerify: d.insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Prepare the upgrade request.
	url := url.URL{
		Host:   addr,
		Scheme: "https",
		Path:   constants.ConnectionUpgradeWebAPI,
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		defer tlsConn.Close()
		return nil, trace.Wrap(err)
	}

	// For now, only ALPN is supported.
	req.Header.Add(constants.ConnectionUpgradeHeader, constants.ConnectionUpgradeTypeALPN)

	// Send the request and checks if upgrade is successful.
	if err = req.Write(tlsConn); err != nil {
		defer tlsConn.Close()
		return nil, trace.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(tlsConn), req)
	if err != nil {
		defer tlsConn.Close()
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if http.StatusSwitchingProtocols != resp.StatusCode {
		defer tlsConn.Close()

		if http.StatusNotFound == resp.StatusCode {
			return nil, trace.NotImplemented(
				"connection upgrade call to %q failed with status code %v. Please upgrade the server and try again.",
				constants.ConnectionUpgradeWebAPI,
				resp.StatusCode,
			)
		}
		return nil, trace.BadParameter("failed to switch Protocols %v", resp.StatusCode)
	}

	// TODO add ping conn
	return tlsConn, nil
}

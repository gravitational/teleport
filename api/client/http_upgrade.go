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
	"fmt"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
)

// isHTTPUpgradeRequired returns true if a tunnel is required through a HTTP
// upgrade.
//
// The function makes a test connection to the Proxy Service and checks if the
// Teleport custom ALPN protocols are preserved. If not, the Proxy Service is
// likely behind an AWS ALB or some custom proxy services that strip out ALPN
// and SNI information on the way to our Proxy Service.
//
// In those cases, the client makes a HTTP "upgrade" call to the Proxy Service
// to establish a tunnel for the origianlly planned traffic.
func isHTTPUpgradeRequired(proxyAddr string, tlsConfig *tls.Config) bool {
	// TODO Currently if remote is not HTTPS, TLS routing is not performed at
	// all. However, the HTTP upgrade is probably a good workaround for HTTP
	// remotes.
	if tlsConfig == nil {
		return false
	}

	// Use an very old/stable protocol for testing to reduce false positives in
	// case remote is running a different version.
	testProtocol := constants.ALPNSNIProtocolReverseTunnel
	testConn, err := tls.Dial("tcp", proxyAddr, &tls.Config{
		NextProtos:         []string{testProtocol, protocolHTTP},
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
	})
	if err != nil {
		// TODO
		logrus.Warn(err)
		return false
	}
	defer testConn.Close()

	// Normally the testProtocol should comme back which indicates the HTTP
	// tunnel is not required.
	//
	// When the testProtocol is lost, double check if regular HTTP is
	// negotiated as there is no point to try a HTTP upgrade if the remote does
	// not serve HTTP.
	return testConn.ConnectionState().NegotiatedProtocol == protocolHTTP
}

// TODO
func doHTTPupgrade(ctx context.Context, proxyAddr string, insecure bool) (*tls.Conn, error) {
	conn, err := tls.Dial("tcp", proxyAddr, &tls.Config{
		InsecureSkipVerify: insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)

	}

	url := url.URL{
		Host:   proxyAddr,
		Scheme: "https",
		Path:   fmt.Sprintf(WebAPIConnectionUpgrade),
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// More on Upgrade headers:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Upgrade
	req.Header.Add(requestHeaderConnection, "upgrade")
	req.Header.Add(requestHeaderUpgrade, UpgradeTypeALPN)

	if err = req.Write(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, trace.BadParameter("failed to switch Protocols %v", resp.StatusCode)
	}

	// TODO add ping conn
	return conn, nil
}

const (
	// WebAPIConnectionUpgrade is the web API to make the HTTP upgrade.
	WebAPIConnectionUpgrade = "/webapi/connectionupgrade"

	// UpgradeTypeALPN is the requested connection upgrade type for server-side
	// ALPN handler to handle the tunneled traffic.
	UpgradeTypeALPN = "alpn"

	// TODO
	requestHeaderUpgrade    = "Upgrade"
	requestHeaderConnection = "Connection"
	protocolHTTP            = "http/1.1"
)

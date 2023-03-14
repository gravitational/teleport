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

package alpnproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// IsALPNConnUpgradeRequired returns true if a tunnel is required through a HTTP
// connection upgrade for ALPN connections.
//
// The function makes a test connection to the Proxy Service and checks if the
// ALPN is supported. If not, the Proxy Service is likely behind an AWS ALB or
// some custom proxy services that strip out ALPN and SNI information on the
// way to our Proxy Service.
//
// In those cases, the Teleport client should make a HTTP "upgrade" call to the
// Proxy Service to establish a tunnel for the originally planned traffic to
// preserve the ALPN and SNI information.
func IsALPNConnUpgradeRequired(addr string, insecure bool) bool {
	netDialer := &net.Dialer{
		Timeout: defaults.DefaultIOTimeout,
	}
	tlsConfig := &tls.Config{
		NextProtos:         []string{string(common.ProtocolReverseTunnel)},
		InsecureSkipVerify: insecure,
	}
	testConn, err := tls.DialWithDialer(netDialer, "tcp", addr, tlsConfig)
	if err != nil {
		// If dialing TLS fails for any reason, we assume connection upgrade is
		// not required so it will fallback to original connection method.
		//
		// This includes handshake failures where both peers support ALPN but
		// no common protocol is getting negotiated. We may have to revisit
		// this situation or make it configurable if we have to get through a
		// middleman with this behavior. For now, we are only interested in the
		// case where the middleman does not support ALPN.
		logrus.Infof("ALPN connection upgrade test failed for %q: %v.", addr, err)
		return false
	}
	defer testConn.Close()

	// Upgrade required when ALPN is not supported on the remote side so
	// NegotiatedProtocol comes back as empty.
	result := testConn.ConnectionState().NegotiatedProtocol == ""
	logrus.Debugf("ALPN connection upgrade required for %q: %v.", addr, result)
	return result
}

// alpnConnUpgradeDialer makes an "HTTP" upgrade call to the Proxy Service then
// tunnels the connection with this connection upgrade.
type alpnConnUpgradeDialer struct {
	dialer    apiclient.ContextDialer
	tlsConfig *tls.Config
}

// newALPNConnUpgradeDialer creates a new alpnConnUpgradeDialer.
func newALPNConnUpgradeDialer(dialer apiclient.ContextDialer, tlsConfig *tls.Config) ContextDialer {
	return &alpnConnUpgradeDialer{
		dialer:    dialer,
		tlsConfig: tlsConfig,
	}
}

// DialContext implements ContextDialer
func (d alpnConnUpgradeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	logrus.Debugf("ALPN connection upgrade for %v.", addr)

	conn, err := d.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// matching the behavior of tls.Dial
	cfg := d.tlsConfig
	if cfg == nil {
		cfg = &tls.Config{}
	}
	if cfg.ServerName == "" {
		colonPos := strings.LastIndex(addr, ":")
		if colonPos == -1 {
			colonPos = len(addr)
		}
		hostname := addr[:colonPos]

		cfg = cfg.Clone()
		cfg.ServerName = hostname
	}

	tlsConn := tls.Client(conn, cfg)

	err = upgradeConnThroughWebAPI(tlsConn, url.URL{
		Host:   addr,
		Scheme: "https",
		Path:   teleport.WebAPIConnUpgrade,
	})
	if err != nil {
		defer tlsConn.Close()
		return nil, trace.Wrap(err)
	}
	return tlsConn, nil
}

func upgradeConnThroughWebAPI(conn net.Conn, api url.URL) error {
	req, err := http.NewRequest(http.MethodGet, api.String(), nil)
	if err != nil {
		return trace.Wrap(err)
	}

	// For now, only "alpn" is supported.
	req.Header.Add(teleport.WebAPIConnUpgradeHeader, teleport.WebAPIConnUpgradeTypeALPN)

	// Send the request and check if upgrade is successful.
	if err = req.Write(conn); err != nil {
		return trace.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	if http.StatusSwitchingProtocols != resp.StatusCode {
		if http.StatusNotFound == resp.StatusCode {
			return trace.NotImplemented(
				"connection upgrade call to %q failed with status code %v. Please upgrade the server and try again.",
				teleport.WebAPIConnUpgrade,
				resp.StatusCode,
			)
		}
		return trace.BadParameter("failed to switch Protocols %v", resp.StatusCode)
	}
	return nil
}

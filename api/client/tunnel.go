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
	"crypto/tls"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
)

// IsHTTPTunnelRequired returns true if a HTTP tunnel is required.
//
// The function makes a test connection to the Proxy Service and checks if the
// Teleport custom ALPN protocols are preserved. If not, it means the Proxy
// Service is likely behind an AWS ALB or some custom proxy services that strip
// out ALPN and/or SNI information on the way to our Proxy Service.
//
// In those cases, the client makes a HTTP "upgrade" call to the Proxy Service
// and tries to establish a tunnel for the origianlly planned connection.
func IsHTTPTunnelRequired(proxyAddr string, insecure bool) bool {
	// Use an very old protocol for testing to reduce false positives in case
	// remote is running an older version.
	testProtocol := constants.ALPNSNIProtocolReverseTunnel
	testConn, err := tls.Dial("tcp", proxyAddr, &tls.Config{
		NextProtos:         []string{testProtocol, protocolHTTP},
		InsecureSkipVerify: insecure,
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
	// negotiated as there is no point to try a HTTP tunnel if the remote does
	// not serve HTTP.
	return testConn.ConnectionState().NegotiatedProtocol == protocolHTTP
}

// TODO
const protocolHTTP = "http/1.1"

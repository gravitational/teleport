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

// Package tlsutils contains utilities for TLS configuration and formats.
package tlsutils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"

	"github.com/gravitational/trace"
)

// ParseCertificatePEM parses PEM-encoded x509 certificate.
func ParseCertificatePEM(bytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("expected PEM-encoded block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	return cert, nil
}

// ContextDialer represents network dialer interface that uses context
type ContextDialer interface {
	// DialContext is a function that dials the specified address
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// TLSDial dials and establishes TLS connection using custom dialer
// is similar to tls.DialWithDialer
// Note: function taken from lib/utils/tlsdial.go
func TLSDial(ctx context.Context, dialer ContextDialer, network, addr string, tlsConfig *tls.Config) (*tls.Conn, error) {
	if tlsConfig == nil {
		return nil, trace.BadParameter("tls config must be specified")
	}

	plainConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	colonPos := strings.LastIndex(addr, ":")
	if colonPos == -1 {
		colonPos = len(addr)
	}
	hostname := addr[:colonPos]

	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	if tlsConfig.ServerName == "" {
		// Make a copy to avoid polluting argument or default.
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = hostname
	}

	conn := tls.Client(plainConn, tlsConfig)
	err = conn.HandshakeContext(ctx)
	if err != nil {
		plainConn.Close()
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

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

package utils

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/trace"
)

// TLSConfig returns default TLS configuration strong defaults.
func TLSConfig(cipherSuites []uint16) *tls.Config {
	config := &tls.Config{}
	SetupTLSConfig(config, cipherSuites)
	return config
}

// SetupTLSConfig sets up cipher suites in existing TLS config
func SetupTLSConfig(config *tls.Config, cipherSuites []uint16) {
	// If ciphers suites were passed in, use them. Otherwise use the
	// Go defaults.
	if len(cipherSuites) > 0 {
		config.CipherSuites = cipherSuites
	}

	config.MinVersion = tls.VersionTLS12
	config.SessionTicketsDisabled = false
	config.ClientSessionCache = tls.NewLRUClientSessionCache(DefaultLRUCapacity)
}

// CipherSuiteMapping transforms Teleport formatted cipher suites strings
// into uint16 IDs.
func CipherSuiteMapping(cipherSuites []string) ([]uint16, error) {
	out := make([]uint16, 0, len(cipherSuites))

	for _, cs := range cipherSuites {
		c, ok := cipherSuiteMapping[cs]
		if !ok {
			return nil, trace.BadParameter("cipher suite not supported: %v", cs)
		}

		out = append(out, c)
	}

	return out, nil
}

// TLSConn is a `net.Conn` that implements some of the functions defined by the
// `tls.Conn` struct. This interface can be used where it could receive a
// `tls.Conn` wrapped in another connection. For example, in the ALPN Proxy,
// some TLS Connections can be wrapped with ping protocol.
type TLSConn interface {
	net.Conn

	// ConnectionState returns basic TLS details about the connection.
	// More info at: https://pkg.go.dev/crypto/tls#Conn.ConnectionState
	ConnectionState() tls.ConnectionState
	// Handshake runs the client or server handshake protocol if it has not yet
	// been run.
	// More info at: https://pkg.go.dev/crypto/tls#Conn.Handshake
	Handshake() error
	// HandshakeContext runs the client or server handshake protocol if it has
	// not yet been run.
	// More info at: https://pkg.go.dev/crypto/tls#Conn.HandshakeContext
	HandshakeContext(context.Context) error
}

// cipherSuiteMapping is the mapping between Teleport formatted cipher
// suites strings and uint16 IDs.
var cipherSuiteMapping = map[string]uint16{
	"tls-rsa-with-aes-128-cbc-sha":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"tls-rsa-with-aes-256-cbc-sha":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"tls-rsa-with-aes-128-cbc-sha256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"tls-rsa-with-aes-128-gcm-sha256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"tls-rsa-with-aes-256-gcm-sha384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"tls-ecdhe-ecdsa-with-aes-128-cbc-sha":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"tls-ecdhe-ecdsa-with-aes-256-cbc-sha":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"tls-ecdhe-rsa-with-aes-128-cbc-sha":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"tls-ecdhe-rsa-with-aes-256-cbc-sha":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"tls-ecdhe-ecdsa-with-aes-128-cbc-sha256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"tls-ecdhe-rsa-with-aes-128-cbc-sha256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"tls-ecdhe-rsa-with-aes-128-gcm-sha256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"tls-ecdhe-ecdsa-with-aes-128-gcm-sha256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"tls-ecdhe-rsa-with-aes-256-gcm-sha384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"tls-ecdhe-ecdsa-with-aes-256-gcm-sha384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"tls-ecdhe-rsa-with-chacha20-poly1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"tls-ecdhe-ecdsa-with-chacha20-poly1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

const (
	// DefaultLRUCapacity is a capacity for LRU session cache
	DefaultLRUCapacity = 1024
	// DefaultCertTTL sets the TTL of the self-signed certificate (1 year)
	DefaultCertTTL = (24 * time.Hour) * 365
)

// DefaultCipherSuites returns the default list of cipher suites that
// Teleport supports. By default Teleport only support modern ciphers
// (Chacha20 and AES GCM) and key exchanges which support perfect forward
// secrecy (ECDHE).
//
// Note that TLS_RSA_WITH_AES_128_GCM_SHA{256,384} have been dropped due to
// being banned by HTTP2 which breaks gRPC clients. For more information see:
// https://tools.ietf.org/html/rfc7540#appendix-A. These two can still be
// manually added if needed.
func DefaultCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,

		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}
}

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

package common

import "golang.org/x/crypto/acme"

// Protocol is the TLS ALPN protocol type.
type Protocol string

const (
	// ProtocolPostgres is TLS ALPN protocol value used to indicate Postgres protocol.
	ProtocolPostgres Protocol = "teleport-postgres"

	// ProtocolMySQL is TLS ALPN protocol value used to indicate MySQL protocol.
	ProtocolMySQL Protocol = "teleport-mysql"

	// ProtocolMongoDB is TLS ALPN protocol value used to indicate Mongo protocol.
	ProtocolMongoDB Protocol = "teleport-mongodb"

	// ProtocolProxySSH is TLS ALPN protocol value used to indicate Proxy SSH protocol.
	ProtocolProxySSH Protocol = "teleport-proxy-ssh"

	// ProtocolReverseTunnel is TLS ALPN protocol value used to indicate Proxy reversetunnel protocol.
	ProtocolReverseTunnel Protocol = "teleport-reversetunnel"

	// ProtocolHTTP is TLS ALPN protocol value used to indicate HTTP2 protocol
	ProtocolHTTP Protocol = "http/1.1"

	// ProtocolHTTP2 is TLS ALPN protocol value used to indicate HTTP2 protocol.
	ProtocolHTTP2 Protocol = "h2"

	// ProtocolDefault is default TLS ALPN value.
	ProtocolDefault Protocol = ""

	// ProtocolAuth allows dialing local/remote auth service based on SNI cluster name value.
	ProtocolAuth Protocol = "teleport-auth@"
)

// SupportedProtocols is the list of supported ALPN protocols.
var SupportedProtocols = []Protocol{
	acme.ALPNProto,
	ProtocolPostgres,
	ProtocolMySQL,
	ProtocolMongoDB,
	ProtocolProxySSH,
	ProtocolReverseTunnel,
	ProtocolHTTP,
	ProtocolHTTP2,
	ProtocolDefault,
	ProtocolAuth,
}

// ProtocolsToString converts the list of Protocols to the list of strings.
func ProtocolsToString(protocols []Protocol) []string {
	out := make([]string, 0, len(protocols))
	for _, v := range protocols {
		out = append(out, string(v))
	}
	return out
}

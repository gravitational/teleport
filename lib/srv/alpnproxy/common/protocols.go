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

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// Protocol is the TLS ALPN protocol type.
type Protocol string

const (
	// ProtocolPostgres is TLS ALPN protocol value used to indicate Postgres protocol.
	ProtocolPostgres Protocol = "teleport-postgres"

	// ProtocolMySQL is TLS ALPN protocol value used to indicate MySQL protocol.
	ProtocolMySQL Protocol = "teleport-mysql"

	// ProtocolMongoDB is TLS ALPN protocol value used to indicate Mongo protocol.
	ProtocolMongoDB Protocol = "teleport-mongodb"

	// ProtocolRedisDB is TLS ALPN protocol value used to indicate Redis protocol.
	ProtocolRedisDB Protocol = "teleport-redis"

	// ProtocolSQLServer is the TLS ALPN protocol value used to indicate SQL Server protocol.
	ProtocolSQLServer Protocol = "teleport-sqlserver"

	// ProtocolSnowflake is TLS ALPN protocol value used to indicate Snowflake protocol.
	ProtocolSnowflake Protocol = "teleport-snowflake"

	// ProtocolProxySSH is TLS ALPN protocol value used to indicate Proxy SSH protocol.
	ProtocolProxySSH Protocol = "teleport-proxy-ssh"

	// ProtocolReverseTunnel is TLS ALPN protocol value used to indicate Proxy reversetunnel protocol.
	ProtocolReverseTunnel Protocol = "teleport-reversetunnel"

	// ProtocolReverseTunnelV2 is TLS ALPN protocol value used to indicate reversetunnel clients
	// that are aware of proxy peering. This is only used on the client side to allow intermediate
	// load balancers to make decisions based on the ALPN header. ProtocolReverseTunnel should still
	// be included in the list of ALPN header for the proxy server to handle the connection properly.
	ProtocolReverseTunnelV2 Protocol = "teleport-reversetunnelv2"

	// ProtocolHTTP is TLS ALPN protocol value used to indicate HTTP2 protocol
	// ProtocolHTTP is TLS ALPN protocol value used to indicate HTTP 1.1 protocol
	ProtocolHTTP Protocol = "http/1.1"

	// ProtocolHTTP2 is TLS ALPN protocol value used to indicate HTTP2 protocol.
	ProtocolHTTP2 Protocol = "h2"

	// ProtocolDefault is default TLS ALPN value.
	ProtocolDefault Protocol = ""

	// ProtocolAuth allows dialing local/remote auth service based on SNI cluster name value.
	ProtocolAuth Protocol = "teleport-auth@"

	// ProtocolProxyGRPC is TLS ALPN protocol value used to indicate gRPC
	// traffic intended for the Teleport proxy.
	ProtocolProxyGRPC Protocol = "teleport-proxy-grpc"

	// ProtocolMySQLWithVerPrefix is TLS ALPN prefix used by tsh to carry
	// MySQL server version.
	ProtocolMySQLWithVerPrefix = Protocol(string(ProtocolMySQL) + "-")

	// ProtocolTCP is TLS ALPN protocol value used to indicate plain TCP connection.
	ProtocolTCP Protocol = "teleport-tcp"
)

// SupportedProtocols is the list of supported ALPN protocols.
var SupportedProtocols = []Protocol{
	ProtocolHTTP2,
	ProtocolHTTP,
	ProtocolPostgres,
	ProtocolMySQL,
	ProtocolMongoDB,
	ProtocolRedisDB,
	ProtocolSQLServer,
	ProtocolSnowflake,
	ProtocolProxySSH,
	ProtocolReverseTunnel,
	ProtocolAuth,
	ProtocolTCP,
}

// ProtocolsToString converts the list of Protocols to the list of strings.
func ProtocolsToString(protocols []Protocol) []string {
	out := make([]string, 0, len(protocols))
	for _, v := range protocols {
		out = append(out, string(v))
	}
	return out
}

// ToALPNProtocol maps provided database protocol to ALPN protocol.
func ToALPNProtocol(dbProtocol string) (Protocol, error) {
	switch dbProtocol {
	case defaults.ProtocolMySQL:
		return ProtocolMySQL, nil
	case defaults.ProtocolPostgres, defaults.ProtocolCockroachDB:
		return ProtocolPostgres, nil
	case defaults.ProtocolMongoDB:
		return ProtocolMongoDB, nil
	case defaults.ProtocolRedis:
		return ProtocolRedisDB, nil
	case defaults.ProtocolSQLServer:
		return ProtocolSQLServer, nil
	case defaults.ProtocolSnowflake:
		return ProtocolSnowflake, nil
	default:
		return "", trace.NotImplemented("%q protocol is not supported", dbProtocol)
	}
}

// IsDBTLSProtocol returns if DB protocol has supported native TLS protocol.
// where connection can be TLS terminated on ALPN proxy side.
// For protocol like MySQL or Postgres where custom TLS implementation is used the incoming
// connection needs to be forwarded to proxy database service where custom TLS handler is invoked
// to terminated DB connection.
func IsDBTLSProtocol(protocol Protocol) bool {
	switch protocol {
	case ProtocolMongoDB, ProtocolRedisDB, ProtocolSQLServer, ProtocolSnowflake:
		return true
	default:
		return false
	}
}

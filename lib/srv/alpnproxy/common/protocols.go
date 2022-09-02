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
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/lib/defaults"
)

const (
	// ProtocolPostgres is TLS ALPN protocol value used to indicate Postgres protocol.
	ProtocolPostgres string = "teleport-postgres"

	// ProtocolMySQL is TLS ALPN protocol value used to indicate MySQL protocol.
	ProtocolMySQL string = "teleport-mysql"

	// ProtocolMongoDB is TLS ALPN protocol value used to indicate Mongo protocol.
	ProtocolMongoDB string = "teleport-mongodb"

	// ProtocolRedisDB is TLS ALPN protocol value used to indicate Redis protocol.
	ProtocolRedisDB string = "teleport-redis"

	// ProtocolSQLServer is the TLS ALPN protocol value used to indicate SQL Server protocol.
	ProtocolSQLServer string = "teleport-sqlserver"

	// ProtocolSnowflake is TLS ALPN protocol value used to indicate Snowflake protocol.
	ProtocolSnowflake string = "teleport-snowflake"

	// ProtocolProxySSH is TLS ALPN protocol value used to indicate Proxy SSH protocol.
	ProtocolProxySSH string = "teleport-proxy-ssh"

	// ProtocolReverseTunnel is TLS ALPN protocol value used to indicate Proxy reversetunnel protocol.
	ProtocolReverseTunnel string = "teleport-reversetunnel"

	// ProtocolReverseTunnelV2 is TLS ALPN protocol value used to indicate reversetunnel clients
	// that are aware of proxy peering. This is only used on the client side to allow intermediate
	// load balancers to make decisions based on the ALPN header. ProtocolReverseTunnel should still
	// be included in the list of ALPN header for the proxy server to handle the connection properly.
	ProtocolReverseTunnelV2 string = "teleport-reversetunnelv2"

	// ProtocolHTTP is TLS ALPN protocol value used to indicate HTTP2 protocol
	// ProtocolHTTP is TLS ALPN protocol value used to indicate HTTP 1.1 protocol
	ProtocolHTTP string = "http/1.1"

	// ProtocolHTTP2 is TLS ALPN protocol value used to indicate HTTP2 protocol.
	ProtocolHTTP2 string = "h2"

	// ProtocolDefault is default TLS ALPN value.
	ProtocolDefault string = ""

	// ProtocolAuth allows dialing local/remote auth service based on SNI cluster name value.
	ProtocolAuth string = "teleport-auth@"

	// ProtocolProxyGRPC is TLS ALPN protocol value used to indicate gRPC
	// traffic intended for the Teleport proxy.
	ProtocolProxyGRPC string = "teleport-proxy-grpc"

	// ProtocolMySQLWithVerPrefix is TLS ALPN prefix used by tsh to carry
	// MySQL server version.
	ProtocolMySQLWithVerPrefix = ProtocolMySQL + "-"

	// ProtocolTCP is TLS ALPN protocol value used to indicate plain TCP connection.
	ProtocolTCP string = "teleport-tcp"

	// ProtocolPingSuffix is TLS ALPN suffix used to wrap connections with
	// Ping.
	ProtocolPingSuffix string = "-ping"
)

// SupportedProtocols is the list of supported ALPN protocols.
var SupportedProtocols = append(
	ProtocolsWithPing(ProtocolsWithPingSupport...),
	append([]string{
		ProtocolHTTP2,
		ProtocolHTTP,
		ProtocolProxySSH,
		ProtocolReverseTunnel,
		ProtocolAuth,
		ProtocolTCP,
	}, DatabaseProtocols...)...,
)

// ToALPNProtocol maps provided database protocol to ALPN protocol.
func ToALPNProtocol(dbProtocol string) (string, error) {
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

var dbProtocols = map[string]bool{
	ProtocolMongoDB:                     true,
	ProtocolRedisDB:                     true,
	ProtocolSQLServer:                   true,
	ProtocolSnowflake:                   true,
	ProtocolWithPing(ProtocolMongoDB):   true,
	ProtocolWithPing(ProtocolRedisDB):   true,
	ProtocolWithPing(ProtocolSQLServer): true,
	ProtocolWithPing(ProtocolSnowflake): true,
}

// IsDBTLSProtocol returns if DB protocol has supported native TLS protocol.
// where connection can be TLS terminated on ALPN proxy side.
// For protocol like MySQL or Postgres where custom TLS implementation is used the incoming
// connection needs to be forwarded to proxy database service where custom TLS handler is invoked
// to terminated DB connection.
func IsDBTLSProtocol(protocol string) bool {
	return dbProtocols[protocol]
}

// DatabaseProtocols is the list of the database protocols supported.
var DatabaseProtocols = []string{
	ProtocolPostgres,
	ProtocolMySQL,
	ProtocolMongoDB,
	ProtocolRedisDB,
	ProtocolSQLServer,
	ProtocolSnowflake,
}

// ProtocolsWithPingSupport is the list of protocols that Ping connection is
// supported. For now, only database protocols are supported.
var ProtocolsWithPingSupport = DatabaseProtocols

// ProtocolsWithPing receives a list a protocols and returns a list of them with
// the Ping protocol suffix.
func ProtocolsWithPing(protocols ...string) []string {
	res := make([]string, len(protocols))
	for i := range res {
		res[i] = ProtocolWithPing(protocols[i])
	}

	return res
}

// ProtocolWithPing receives a protocol and returns it with the Ping protocol
// suffix.
func ProtocolWithPing(protocol string) string {
	return protocol + ProtocolPingSuffix
}

// IsPingProtocol checks if the provided protocol is suffixed with Ping.
func IsPingProtocol(protocol string) bool {
	return strings.HasSuffix(protocol, ProtocolPingSuffix)
}

// HasPingSupport checks if the provided protocol supports Ping protocol.
func HasPingSupport(protocol string) bool {
	return slices.Contains(ProtocolsWithPingSupport, protocol)
}

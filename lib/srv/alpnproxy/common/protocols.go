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

package common

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
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

	// ProtocolOracle is TLS ALPN protocol value used to indicate Oracle protocol.
	ProtocolOracle Protocol = "teleport-oracle"

	// ProtocolRedisDB is TLS ALPN protocol value used to indicate Redis protocol.
	ProtocolRedisDB Protocol = "teleport-redis"

	// ProtocolSQLServer is the TLS ALPN protocol value used to indicate SQL Server protocol.
	ProtocolSQLServer Protocol = "teleport-sqlserver"

	// ProtocolSnowflake is TLS ALPN protocol value used to indicate Snowflake protocol.
	ProtocolSnowflake Protocol = "teleport-snowflake"

	// ProtocolCassandra is the TLS ALPN protocol value used to indicate Cassandra protocol.
	ProtocolCassandra Protocol = "teleport-cassandra"

	// ProtocolElasticsearch is TLS ALPN protocol value used to indicate Elasticsearch protocol.
	ProtocolElasticsearch Protocol = "teleport-elasticsearch"

	// ProtocolOpenSearch is TLS ALPN protocol value used to indicate OpenSearch protocol.
	ProtocolOpenSearch Protocol = "teleport-opensearch"

	// ProtocolDynamoDB is TLS ALPN protocol value used to indicate DynamoDB protocol.
	ProtocolDynamoDB Protocol = "teleport-dynamodb"

	// ProtocolClickhouse is TLS ALPN protocol value used to indicate Clickhouse Protocol.
	ProtocolClickhouse Protocol = "teleport-clickhouse"

	// ProtocolSpanner is TLS ALPN protocol value used to indicate Google Spanner (gRPC) Protocol.
	ProtocolSpanner Protocol = "teleport-spanner"

	// ProtocolProxySSH is TLS ALPN protocol value used to indicate Proxy SSH protocol.
	ProtocolProxySSH Protocol = "teleport-proxy-ssh"

	// ProtocolProxySSHGRPC is TLS ALPN protocol value used to indicate gRPC
	// traffic intended for the Teleport Proxy on the SSH port.
	ProtocolProxySSHGRPC Protocol = "teleport-proxy-ssh-grpc"

	// ProtocolReverseTunnel is TLS ALPN protocol value used to indicate Proxy reversetunnel protocol.
	ProtocolReverseTunnel Protocol = "teleport-reversetunnel"

	// ProtocolReverseTunnelV2 is TLS ALPN protocol value used to indicate reversetunnel clients
	// that are aware of proxy peering. This is only used on the client side to allow intermediate
	// load balancers to make decisions based on the ALPN header. ProtocolReverseTunnel should still
	// be included in the list of ALPN header for the proxy server to handle the connection properly.
	ProtocolReverseTunnelV2 Protocol = "teleport-reversetunnelv2"

	// ProtocolHTTP is TLS ALPN protocol value used to indicate HTTP 1.1 protocol
	ProtocolHTTP Protocol = "http/1.1"

	// ProtocolHTTP2 is TLS ALPN protocol value used to indicate HTTP2 protocol.
	ProtocolHTTP2 Protocol = "h2"

	// ProtocolDefault is default TLS ALPN value.
	ProtocolDefault Protocol = ""

	// ProtocolAuth allows dialing local/remote auth service based on SNI cluster name value.
	ProtocolAuth Protocol = "teleport-auth@"

	// ProtocolProxyGRPCInsecure is TLS ALPN protocol value used to indicate gRPC
	// traffic intended for the Teleport proxy join service.
	// Credentials are not verified since this is used for node joining.
	ProtocolProxyGRPCInsecure Protocol = "teleport-proxy-grpc"

	// ProtocolProxyGRPCSecure is TLS ALPN protocol value used to indicate gRPC
	// traffic intended for the Teleport proxy service with mTLS authentication.
	ProtocolProxyGRPCSecure Protocol = "teleport-proxy-grpc-mtls"

	// ProtocolMySQLWithVerPrefix is TLS ALPN prefix used by tsh to carry
	// MySQL server version.
	ProtocolMySQLWithVerPrefix = Protocol(string(ProtocolMySQL) + "-")

	// ProtocolTCP is TLS ALPN protocol value used to indicate plain TCP connection.
	ProtocolTCP Protocol = "teleport-tcp"

	// ProtocolPingSuffix is TLS ALPN suffix used to wrap connections with
	// Ping.
	ProtocolPingSuffix Protocol = "-ping"
)

// SupportedProtocols is the list of supported ALPN protocols.
var SupportedProtocols = WithPingProtocols(
	append([]Protocol{
		// HTTP needs to be prioritized over HTTP2 due to a bug in Chrome:
		// https://bugs.chromium.org/p/chromium/issues/detail?id=1379017
		// If Chrome resolves this, we can switch the prioritization. We may
		// also be able to get around this if https://github.com/golang/go/issues/49918
		// is implemented and we can enable HTTP2 websockets on our end, but
		// it's less clear this will actually fix the issue.
		ProtocolHTTP,
		ProtocolHTTP2,
		ProtocolProxySSH,
		ProtocolReverseTunnel,
		ProtocolAuth,
		ProtocolTCP,
		ProtocolProxySSHGRPC,
		ProtocolProxyGRPCInsecure,
		ProtocolProxyGRPCSecure,
	}, DatabaseProtocols...),
)

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
	case defaults.ProtocolOracle:
		return ProtocolOracle, nil
	case defaults.ProtocolRedis:
		return ProtocolRedisDB, nil
	case defaults.ProtocolSQLServer:
		return ProtocolSQLServer, nil
	case defaults.ProtocolSnowflake:
		return ProtocolSnowflake, nil
	case defaults.ProtocolCassandra:
		return ProtocolCassandra, nil
	case defaults.ProtocolElasticsearch:
		return ProtocolElasticsearch, nil
	case defaults.ProtocolOpenSearch:
		return ProtocolOpenSearch, nil
	case defaults.ProtocolDynamoDB:
		return ProtocolDynamoDB, nil
	case defaults.ProtocolClickHouse, defaults.ProtocolClickHouseHTTP:
		return ProtocolClickhouse, nil
	case defaults.ProtocolSpanner:
		return ProtocolSpanner, nil
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
	dbTLSProtocols := []Protocol{
		ProtocolMongoDB,
		ProtocolOracle,
		ProtocolRedisDB,
		ProtocolSQLServer,
		ProtocolSnowflake,
		ProtocolCassandra,
		ProtocolElasticsearch,
		ProtocolOpenSearch,
		ProtocolDynamoDB,
		ProtocolClickhouse,
		ProtocolSpanner,
	}

	return slices.ContainsFunc(dbTLSProtocols, func(dbTLSProtocol Protocol) bool {
		return protocol == dbTLSProtocol || protocol == ProtocolWithPing(dbTLSProtocol)
	})
}

// DatabaseProtocols is the list of the database protocols supported.
var DatabaseProtocols = []Protocol{
	ProtocolPostgres,
	ProtocolMySQL,
	ProtocolMongoDB,
	ProtocolOracle,
	ProtocolRedisDB,
	ProtocolSQLServer,
	ProtocolSnowflake,
	ProtocolCassandra,
	ProtocolElasticsearch,
	ProtocolOpenSearch,
	ProtocolDynamoDB,
	ProtocolClickhouse,
	ProtocolSpanner,
}

// ProtocolsWithPingSupport is the list of protocols that Ping connection is
// supported. For now, only database protocols are supported.
var ProtocolsWithPingSupport = append(
	DatabaseProtocols,
	ProtocolTCP,
)

// WithPingProtocols adds Ping protocols to the list for each protocol that
// supports Ping.
func WithPingProtocols(protocols []Protocol) []Protocol {
	var pingProtocols []Protocol
	for _, protocol := range protocols {
		if HasPingSupport(protocol) {
			pingProtocols = append(pingProtocols, ProtocolWithPing(protocol))
		}
	}
	return utils.Deduplicate(append(pingProtocols, protocols...))
}

// ProtocolWithPing receives a protocol and returns it with the Ping protocol
// suffix.
func ProtocolWithPing(protocol Protocol) Protocol {
	return Protocol(string(protocol) + string(ProtocolPingSuffix))
}

// IsPingProtocol checks if the provided protocol is suffixed with Ping.
func IsPingProtocol(protocol Protocol) bool {
	return strings.HasSuffix(string(protocol), string(ProtocolPingSuffix))
}

// HasPingSupport checks if the provided protocol supports Ping protocol.
func HasPingSupport(protocol Protocol) bool {
	return slices.Contains(ProtocolsWithPingSupport, protocol)
}

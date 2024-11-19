package srvconnect

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// DatabaseServersGetter is an interface for retrieving information about
// database proxy servers within a specific namespace.
type DatabaseServersGetter interface {
	// GetDatabaseServers returns all registered database proxy servers.
	GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error)
}

// Cluster represents a remote site the behavior of a cluster in the system,
// providing methods to retrieve its name and access point.
type Cluster interface {
	// GetName returns site name (identified by authority domain's name).
	GetName() string
}

// GetDatabaseServersParams contains the parameters required to retrieve
// database servers from a specific cluster.
type GetDatabaseServersParams struct {
	Logger *slog.Logger
	// Cluster represents the cluster to which the database belongs.
	Cluster Cluster
	// DatabaseServersGetter used to fetch the list of database servers.
	DatabaseServersGetter DatabaseServersGetter
	// Identity contains the identity information.
	Identity tlsca.Identity
}

// GetDatabaseServers returns a list of database servers in a cluster that match
// the routing information from the provided identity.
func GetDatabaseServers(ctx context.Context, params GetDatabaseServersParams) ([]types.DatabaseServer, error) {
	servers, err := params.DatabaseServersGetter.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	params.Logger.DebugContext(ctx, "Available database servers.", "cluster", params.Cluster.GetName(), "servers", servers)

	// Find out which database servers proxy the database a user is
	// connecting to using routing information from identity.
	var result []types.DatabaseServer
	for _, server := range servers {
		if server.GetDatabase().GetName() == params.Identity.RouteToDatabase.ServiceName {
			result = append(result, server)
		}
	}

	if len(result) != 0 {
		return result, nil
	}

	return nil, trace.NotFound("database %q not found among registered databases in cluster %q",
		params.Identity.RouteToDatabase.ServiceName,
		params.Identity.RouteToCluster)
}

// DatabaseCertificateSigner defines an interface for signing database
// Certificate Signing Requests (CSRs).
type DatabaseCertificateSigner interface {
	// SignDatabaseCSR generates a client certificate used by proxy when talking
	// to a remote database service.
	SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error)
}

// AuthPreferenceGetter is an interface for retrieving the current configured
// cluster auth preference.
type AuthPreferenceGetter interface {
	// GetAuthPreference returns the current cluster auth preference.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
}

// ServerTLSConfigParams contains the parameters required to configure
// a TLS connection to a database server.
type ServerTLSConfigParams struct {
	// CertSigner is the interface used to sign certificate signing requests
	// for establishing a secure TLS connection.
	CertSigner DatabaseCertificateSigner
	// AuthPreference provides the authentication preference configuration
	// used to determine cryptographic settings for certificate generation.
	AuthPreference AuthPreferenceGetter
	// Server represents the database server for which the TLS configuration
	// is being generated.
	Server types.DatabaseServer
	// Identity contains the identity information.
	Identity tlsca.Identity
}

// GetServerTLSConfig returns TLS config used for establishing connection
// to a remote database server over reverse tunnel.
func GetServerTLSConfig(ctx context.Context, params ServerTLSConfigParams) (*tls.Config, error) {
	privateKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(params.AuthPreference),
		cryptosuites.ProxyToDatabaseAgent)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subject, err := params.Identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := params.CertSigner.SignDatabaseCSR(ctx, &proto.DatabaseCSRRequest{
		CSR:         csr,
		ClusterName: params.Identity.RouteToCluster,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := keys.TLSCertificateForSigner(privateKey, response.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	for _, caCert := range response.CACerts {
		ok := pool.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, trace.BadParameter("failed to append CA certificate")
		}
	}

	return &tls.Config{
		ServerName:   params.Server.GetHostname(),
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// ShuffleFunc defines a function that shuffles a list of database servers.
type ShuffleFunc func([]types.DatabaseServer) []types.DatabaseServer

// ShuffleSort is a ShuffleFunc that sorts database servers by name and host ID.
// Used to provide predictable behavior in tests.
func ShuffleSort(servers []types.DatabaseServer) []types.DatabaseServer {
	sort.Sort(types.DatabaseServers(servers))
	return servers
}

// ShuffleRandom is a ShuffleFunc that randomizes the order of database servers.
// Used to provide load balancing behavior when proxying to multiple agents.
func ShuffleRandom(servers []types.DatabaseServer) []types.DatabaseServer {
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(
		len(servers), func(i, j int) {
			servers[i], servers[j] = servers[j], servers[i]
		})
	return servers
}

type Dialer interface {
	// Dial dials any address within the site network, in terminating
	// mode it uses local instance of forwarding server to terminate
	// and record the connection.
	Dial(params reversetunnelclient.DialParams) (conn net.Conn, err error)
}

// ConnectParams contains parameters for connecting to the database server.
type ConnectParams struct {
	Logger *slog.Logger
	// Identity contains the identity information.
	Identity tlsca.Identity
	// Servers is the list of database servers that can handle the connection.
	Servers []types.DatabaseServer
	// ShuffleFunc is a function used to shuffle the list of database servers.
	ShuffleFunc ShuffleFunc
	// Cluster represents the cluster to which the database belongs.
	Cluster Cluster
	// Cluster represents the cluster to which the database belongs.
	Dialer Dialer
	// CertSigner is used to sign certificates for authenticating with the
	// database.
	CertSigner DatabaseCertificateSigner
	// AuthPreference provides the authentication preferences for the cluster.
	AuthPreference AuthPreferenceGetter
	// ClientSrcAddr is the source address of the client making the connection.
	ClientSrcAddr net.Addr
	// ClientDstAddr is the destination address of the client making the
	// connection.
	ClientDstAddr net.Addr
}

// ConnectStats contains statistics about the connection attempts.
type ConnectStats struct {
	// AttemptedServers is the number of database servers that were attempted to
	// connect to.
	AttemptedServers int
	// DialAttempts is the number of times a dial to a server was attempted.
	DialAttempts int
	// DialFailures is the number of times a dial to a server failed.
	DialFailures int
}

// Connect connects to the database server running on a remote cluster
// over reverse tunnel and upgrades this end of the connection to TLS so
// the identity can be passed over it.
func Connect(ctx context.Context, params ConnectParams) (net.Conn, ConnectStats, error) {
	if params.ShuffleFunc == nil {
		params.ShuffleFunc = ShuffleRandom
	}

	stats := ConnectStats{}

	// There may be multiple database servers proxying the same database. If
	// we get a connection problem error trying to dial one of them, likely
	// the database server is down so try the next one.
	for _, server := range params.ShuffleFunc(params.Servers) {
		stats.AttemptedServers++
		params.Logger.DebugContext(ctx, "Dialing to database service.", "server", server)
		tlsConfig, err := GetServerTLSConfig(ctx, ServerTLSConfigParams{
			AuthPreference: params.AuthPreference,
			CertSigner:     params.CertSigner,
			Identity:       params.Identity,
			Server:         server,
		})
		if err != nil {
			return nil, stats, trace.Wrap(err)
		}

		stats.DialAttempts++
		serviceConn, err := params.Dialer.Dial(reversetunnelclient.DialParams{
			From:                  params.ClientSrcAddr,
			To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: reversetunnelclient.LocalNode},
			OriginalClientDstAddr: params.ClientDstAddr,
			ServerID:              fmt.Sprintf("%v.%v", server.GetHostID(), params.Cluster.GetName()),
			ConnType:              types.DatabaseTunnel,
			ProxyIDs:              server.GetProxyIDs(),
		})
		if err != nil {
			stats.DialFailures++
			// If an agent is down, we'll retry on the next one (if available).
			if isReverseTunnelDownError(err) {
				params.Logger.WarnContext(ctx, "Failed to dial database service.", "server", server, "error", err)
				continue
			}
			return nil, stats, trace.Wrap(err)
		}
		// Upgrade the connection so the client identity can be passed to the
		// remote server during TLS handshake. On the remote side, the connection
		// received from the reverse tunnel will be handled by tls.Server.
		serviceConn = tls.Client(serviceConn, tlsConfig)
		return serviceConn, stats, nil
	}

	return nil, stats, trace.BadParameter("failed to connect to any of the database servers")
}

// isReverseTunnelDownError returns true if the provided error indicates that
// the reverse tunnel connection is down e.g. because the agent is down.
func isReverseTunnelDownError(err error) bool {
	return trace.IsConnectionProblem(err) ||
		strings.Contains(err.Error(), reversetunnelclient.NoDatabaseTunnel)
}

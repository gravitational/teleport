package benchmark

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/benchmark/db"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

// DBBenchmarkConfig common configuration used by database benchmark suites.
type DBBenchmarkConfig struct {
	// DBService database service name of the target database.
	DBService string
	// DBUser database user used to connect to the target database.
	DBUser string
	// DBName database name where the benchmark queries are going to be
	// executed.
	DBName string
	// InsecureSkipVerify bypasses verification of TLS certificate when
	// talking to database.
	InsecureSkipVerify bool
	// URI is the direct database connection URI.
	URI string
}

// CheckAndSetDefaults validates configuration and set default values.
func (c *DBBenchmarkConfig) CheckAndSetDefaults() error {
	if c.URI == "" && c.DBService != "" {
		return trace.BadParameter("database or direct database URI must be provided")
	}

	return nil
}

// DBConnectBenchmark is a benchmark suites that connects to the target database.
type DBConnectBenchmark struct {
	Config DBBenchmarkConfig
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (d DBConnectBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (WorkloadFunc, error) {
	if err := d.Config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	connConfig, err := retrieveDatabaseConnectConfig(ctx, tc, d.Config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		conn, err := connectToDatabase(ctx, connConfig)
		if err != nil {
			return trace.Wrap(err)
		}
		defer conn.Close(ctx)

		return trace.Wrap(conn.Ping(ctx))
	}, nil
}

// getDatabase loads the database which the name matches.
func getDatabase(ctx context.Context, tc *client.TeleportClient, serviceName string) (types.Database, error) {
	databases, err := tc.ListDatabases(ctx, &proto.ListResourcesRequest{
		Namespace:           tc.Namespace,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, serviceName),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(databases) != 1 {
		return nil, trace.NotFound("no databases with name %q found", serviceName)
	}

	return databases[0], nil
}

// retrieveDatabaseConnectConfig generates the necessary configuration to
// connect to the target database.
func retrieveDatabaseConnectConfig(ctx context.Context, tc *client.TeleportClient, config DBBenchmarkConfig) (*db.DatabaseConnectionConfig, error) {
	if config.URI != "" {
		protocol, err := extractProtocolFromURI(config.URI)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &db.DatabaseConnectionConfig{
			Protocol: protocol,
			URI:      config.URI,
		}, nil
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database, err := getDatabase(ctx, tc, config.DBService)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := tc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToCluster: tc.SiteName,
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: database.GetName(),
			Protocol:    database.GetProtocol(),
			Username:    config.DBUser,
			Database:    config.DBName,
		},
		AccessRequests: profile.ActiveRequests.AccessRequests,
	}, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawCert, ok := key.DBTLSCerts[config.DBService]
	if !ok {
		return nil, trace.AccessDenied("failed to retrieve database certificates")
	}
	tlsCert, err := key.TLSCertificate(rawCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certPool := x509.NewCertPool()
	for _, caCert := range key.TLSCAs() {
		cert, err := tlsca.ParseCertificatePEM(caCert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool.AddCert(cert)
	}

	host, port := tc.DatabaseProxyHostPort(tlsca.RouteToDatabase{Protocol: database.GetProtocol()})
	return &db.DatabaseConnectionConfig{
		Protocol:     database.GetProtocol(),
		Username:     config.DBUser,
		Database:     config.DBName,
		ProxyAddress: net.JoinHostPort(host, strconv.Itoa(port)),
		TLSConfig: &tls.Config{
			RootCAs:            certPool,
			Certificates:       []tls.Certificate{tlsCert},
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}, nil
}

// extractProtocolFromURI receives a database URI and returns the database
// protocol.
func extractProtocolFromURI(uri string) (string, error) {
	if strings.HasPrefix(uri, "postgres://") {
		return types.DatabaseProtocolPostgreSQL, nil
	}

	return "", trace.BadParameter("unable to define database protocol for URI %q", uri)
}

// connectToDatabase connects and return a DatabaseClient using the
// configuration provided.
func connectToDatabase(ctx context.Context, config *db.DatabaseConnectionConfig) (db.DatabaseClient, error) {
	switch config.Protocol {
	case types.DatabaseProtocolPostgreSQL:
		return db.ConnectPostgres(ctx, config)
	}

	return nil, trace.BadParameter("%q database protocol is not supported", config.Protocol)
}

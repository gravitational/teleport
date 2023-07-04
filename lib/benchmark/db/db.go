package db

import (
	"context"
	"crypto/tls"
)

// DatabaseClient represents a database connection.
type DatabaseClient interface {
	// Ping runs a command on the database to ensure that the connection is
	// alive.
	Ping(context.Context) error
	// Close closes the connection.
	Close(context.Context) error
	// Query runs a query on the database.
	Query(context.Context, string) error
}

// DatabaseConnectionConfig contains all information necessary to establish a
// new database connection.
type DatabaseConnectionConfig struct {
	// Protocol database protocol.
	Protocol string
	// URI direct database connection URI.
	URI string
	// Username database username the connection should use.
	Username string
	// Database database name where the connection should point to.
	Database string
	// ProxyAddress Teleport database proxy address.
	ProxyAddress string
	// TLSConfig TLS configuration containing Teleport CA and database
	// certificates.
	TLSConfig *tls.Config
}

// ConnectFunc is a function that establishes a database connection and returns the
// DatabaseClient.
type ConnectFunc func(ctx context.Context, config *DatabaseConnectionConfig) (DatabaseClient, error)

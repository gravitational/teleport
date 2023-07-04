package db

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
)

// ConnectPostgres connects to a PostgreSQL database.
func ConnectPostgres(ctx context.Context, config *DatabaseConnectionConfig) (DatabaseClient, error) {
	pgconnConfig, err := getConnectionConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pgConn, err := pgconn.ConnectConfig(ctx, pgconnConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &postgresClient{pgConn}, nil
}

// getConnectionConfig generates the postgres connection configuration struct.
func getConnectionConfig(config *DatabaseConnectionConfig) (*pgconn.Config, error) {
	if config.URI != "" {
		return pgconn.ParseConfig(config.URI)
	}

	pgconnConfig, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%v/?sslmode=verify-full", config.ProxyAddress))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgconnConfig.User = config.Username
	pgconnConfig.Database = config.Database
	pgconnConfig.TLSConfig = config.TLSConfig
	return pgconnConfig, nil
}

var _ (DatabaseClient) = (*postgresClient)(nil)

// postgresClient implements DatabaseClient for PostgreSQL connections.
type postgresClient struct {
	conn *pgconn.PgConn
}

// Ping runs a command on the database to ensure that connection is alive.
func (p *postgresClient) Ping(ctx context.Context) error {
	return trace.Wrap(p.Query(ctx, "SELECT 1;"))
}

// Close closes the connection.
func (p *postgresClient) Close(ctx context.Context) error {
	return trace.Wrap(p.conn.Close(ctx))
}

func (p *postgresClient) Query(ctx context.Context, query string) error {
	_, err := p.conn.Exec(ctx, query).ReadAll()
	return trace.Wrap(err)
}

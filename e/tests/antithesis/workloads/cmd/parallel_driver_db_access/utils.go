package main

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
)

type databaseQueryResult struct {
	ServiceName string
	SeedCount   int
	Marker      string
	Source      string
}

func resolveDatabase(ctx context.Context, clt *client.TeleportClient, target DatabaseTarget) (types.Database, error) {

	filter := clt.ResourceFilter(types.KindDatabaseServer)
	filter.Labels = target.labels()
	filter.PredicateExpression = target.PredicateExpression

	databases, err := clt.ListDatabases(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err, "listing databases")
	}

	if !target.UsesResourceMatcher() {
		// If we are not using labels / predicates then all databases will be returned here
		// return first first one matching the name.
		for _, db := range databases {
			if target.Matches(db) {
				return db, nil
			}
		}

		return nil, trace.NotFound("database target not found")
	}

	switch len(databases) {
	case 0:
		return nil, trace.NotFound("database target not found")
	case 1:
		return databases[0], nil
	default:
		return nil, trace.BadParameter("More than one database matched the filter (this is a bug)")
	}
}

func queryDatabase(ctx context.Context, clt *client.TeleportClient, route proto.RouteToDatabase, marker string) (*databaseQueryResult, error) {
	conn, err := connectPostgres(ctx, clt, route)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to postgres")
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := conn.Close(closeCtx); err != nil {
			slog.WarnContext(ctx, "failed to close postgres connection", "error", err)
		}
	}()

	var seedCount int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM antithesis.seed_items WHERE active`).Scan(&seedCount); err != nil {
		return nil, trace.ConnectionProblem(err, "querying seeded rows")
	}

	var source string
	markerQuery := `/* antithesis-db-access:` + marker + ` */
INSERT INTO antithesis.workload_markers(marker, source)
VALUES ($1, $2)
ON CONFLICT (marker) DO UPDATE
SET source = EXCLUDED.source, updated_at = now()
RETURNING source`
	if err := conn.QueryRow(ctx, markerQuery, marker, route.GetServiceName()).Scan(&source); err != nil {
		return nil, trace.ConnectionProblem(err, "writing marker row")
	}

	return &databaseQueryResult{
		ServiceName: route.GetServiceName(),
		SeedCount:   seedCount,
		Marker:      marker,
		Source:      source,
	}, nil
}

func connectPostgres(ctx context.Context, clt *client.TeleportClient, route proto.RouteToDatabase) (*pgx.Conn, error) {
	// Passing postgres://foo returns a default config which we can override.
	config, err := pgx.ParseConfig("postgres://teleport-postgres")
	if err != nil {
		return nil, trace.Wrap(err, "building postgres connection config")
	}

	config.User = route.GetUsername()
	config.Database = route.GetDatabase()
	config.ConnectTimeout = defaults.DatabaseConnectTimeout
	config.RuntimeParams = map[string]string{
		// We set the app name to enable it in the logs, runtime queries etc.
		"application_name": "antithesis-db-access",
	}
	config.TLSConfig = nil
	config.Fallbacks = nil
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.LookupFunc = func(context.Context, string) ([]string, error) {
		return []string{"teleport-postgres"}, nil // Dummy value, dialer ignores it.
	}
	config.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
		// Dial via Teleport client.
		conn, err := clt.DialDatabase(ctx, route)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "dialing database")
	}
	return conn, nil
}

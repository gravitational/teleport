package main

import (
	"context"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	workloadclient "github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/client"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
)

func runAllowedDatabaseAccessProperty(ctx context.Context, params *TestCaseParams) error {
	clt, cleanup, err := workloadclient.NewTeleportClient(ctx, workloadclient.TeleportClientConfig{
		Identity:            params.Identity,
		Host:                params.Target.name(),
		Labels:              params.Target.labels(),
		PredicateExpression: params.Target.PredicateExpression,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()

	db, err := resolveDatabase(ctx, clt, params.Target)
	details := params.Details(map[string]any{
		"error": err,
	})
	assert.Sometimes(err == nil,
		"An authorized identity can resolve a database target",
		details)
	if err != nil {
		return trace.Wrap(err, "resolving database target")
	}

	route := params.routeFor(db)
	marker, err := uuid.NewV7()
	if err != nil {
		return trace.Wrap(err, "generating marker")
	}

	start := time.Now().Add(-testenv.MaxTolerableClockJitter)
	result, err := queryDatabase(ctx, clt, route, marker.String())
	// Padding to account for delay between session ending and the event being emitted.
	end := time.Now().Add(testenv.AuditEventEmitDeadline + testenv.MaxTolerableClockJitter)

	details = params.Details(map[string]any{
		"database": db.GetName(),
		"protocol": db.GetProtocol(),
		"uri":      db.GetURI(),
		"route":    route,
		"result":   result,
		"error":    err,
	})
	assert.Sometimes(err == nil,
		"Sometimes an authorized identity can run PostgreSQL queries through Teleport Database Access",
		details)
	if err != nil {
		return trace.Wrap(err, "querying database")
	}

	wantSeedRows := 2
	assert.AlwaysOrUnreachable(result.SeedCount >= wantSeedRows,
		"Every reachable authorized PostgreSQL query can read the seeded fixture table",
		params.Details(map[string]any{
			"got_seed_count":  result.SeedCount,
			"want_seed_count": wantSeedRows,
		}))
	if result.SeedCount < wantSeedRows {
		return trace.BadParameter("unexpected seeded row count: %d", result.SeedCount)
	}

	assert.AlwaysOrUnreachable(result.Source == db.GetName(),
		"Every reachable authorized PostgreSQL query can write and read a workload marker",
		params.Details(map[string]any{
			"got_source":  result.Source,
			"want_source": db.GetName(),
			"marker":      marker.String(),
		}))
	if result.Source != db.GetName() {
		return trace.BadParameter("unexpected marker source: %q", result.Source)
	}

	return trace.Wrap(params.assertDatabaseAccessAuditEvents(ctx, marker.String(), route, start, end))
}

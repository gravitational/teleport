package main

import (
	"context"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	workloadclient "github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/client"
)

func runDeniedDatabaseAccessProperty(ctx context.Context, params *TestCaseParams) error {
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

	marker, err := uuid.NewV7()
	if err != nil {
		return trace.Wrap(err, "generating marker")
	}

	// a denied identity cannot list database servers, so resolving would fail before a
	// connection is ever attempted.
	route := params.staticRoute()
	if route.GetServiceName() == "" {
		return trace.BadParameter("deny test cases must target a database by name")
	}

	_, err = queryDatabase(ctx, clt, route, marker.String())
	details := params.Details(map[string]any{
		"database": route.GetServiceName(),
		"route":    route,
		"error":    err,
		"marker":   marker.String(),
	})
	denied := trace.IsAccessDenied(err) || trace.IsConnectionProblem(err) || trace.IsNotFound(err)
	assert.AlwaysOrUnreachable(denied, "Denied identity cannot query database target", details)
	if err == nil {
		return trace.Errorf("database query unexpectedly succeeded")
	}
	if !denied {
		return trace.Wrap(err, "querying database")
	}

	return nil
}

// runDeniedDatabaseResolutionProperty asserts that a denied identity cannot
// even discover the target database.
func runDeniedDatabaseResolutionProperty(ctx context.Context, params *TestCaseParams) error {
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
	if err == nil {
		details["database"] = db.GetName()
	}

	assert.Sometimes(!trace.IsConnectionProblem(err), "Sometimes denied identity access does not result in a network error when resolving database target", details)
	if trace.IsConnectionProblem(err) {
		return nil
	}

	denied := trace.IsNotFound(err) || trace.IsAccessDenied(err)
	assert.AlwaysOrUnreachable(denied, "Denied identity cannot resolve database target", details)
	if err == nil {
		return trace.Errorf("database target %q unexpectedly resolved", db.GetName())
	}
	if !denied {
		return trace.Wrap(err, "resolving database target")
	}

	return nil
}

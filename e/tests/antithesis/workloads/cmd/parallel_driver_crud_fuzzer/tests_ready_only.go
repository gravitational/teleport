package main

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
)

func runReadOnlyReadProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up")
	}
	defer cleanup(ctx)

	resource, err := setupResource(ctx, adminOps, nil)
	if err != nil {
		return trace.Wrap(err, "getting resource")
	}
	defer cleanupResource(ctx, adminOps, resource)

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Read-only identity can read an existing resource",
		Timeout: writeReplicationTimeout,
		Details: params.Details(map[string]any{"name": resource.GetMetadata().GetName()}),
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, resource.GetMetadata().GetName())
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}

			return equalResourceAfterServerAssignments(ops, resource, r), nil
		},
	})
}

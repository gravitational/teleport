package main

import (
	"context"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/gravitational/trace"
)

func runDenyReadProperty(ctx context.Context, params *TestCaseParams) error {
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

	name := resource.GetMetadata().GetName()
	details := params.Details(map[string]any{"name": name})
	_, err = ops.Get153(ctx, name)
	details["error"] = err

	denied := trace.IsAccessDenied(err) || trace.IsNotFound(err)
	assert.AlwaysOrUnreachable(denied, "Denied identity cannot read an existing resource", details)
	return nil
}

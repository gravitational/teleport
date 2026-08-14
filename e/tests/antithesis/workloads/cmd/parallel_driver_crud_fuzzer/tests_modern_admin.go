package main

import (
	"context"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/crud"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
)

func runReadYourWritesModernProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	resource, err := ops.NewResource(fuzzerPrefix + "-" + uuid.New().String())
	if err != nil {
		return trace.Wrap(err, "creating new resource")
	}
	defer cleanupResource(ctx, adminOps, resource)

	name := resource.GetMetadata().GetName()
	details := params.Details(map[string]any{"name": name})

	created, err := ops.Create(ctx, resource)
	// Hint to the fuzzer that we want cases where this succeeds to continue with the assertion.
	assert.Sometimes(err == nil, "Sometimes writing a resource for read-your-writes succeeds", details)
	if err != nil {
		return trace.Wrap(err, "creating resource")
	}

	normalized := normalizeResourceAfterServerAssignments(ops, resource, created)
	equal := ops.Equal(normalized, created)
	assert.Always(equal,
		"Modern create semantics return requested object with server-assigned metadata when successful",
		details)

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Reading a written resource returns the created value",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, name)
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}
			return ops.Equal(created, r), nil
		},
	})
}

func runReadYourUpdatesModernProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	current, err := setupResource(ctx, adminOps, map[string]string{valueLabel: "old"})
	if err != nil {
		return trace.Wrap(err, "setting up resource")
	}
	defer cleanupResource(ctx, adminOps, current)
	name := current.GetMetadata().GetName()

	newValue := "new-" + uuid.New().String()
	details := params.Details(map[string]any{
		"name":       name,
		"want_value": newValue,
		"old_value":  current.GetMetadata().GetLabels()[valueLabel],
	})

	if err := crud.SetStaticLabel(current, valueLabel, newValue); err != nil {
		return trace.Wrap(err, "setting label")
	}

	updated, err := ops.Update(ctx, current)
	assert.Sometimes(err == nil, "Sometimes updating a resource for read-your-updates succeeds", details)
	if err != nil {
		return trace.Wrap(err, "updating resource")
	}

	normalized := normalizeResourceAfterServerAssignments(ops, current, updated)
	equal := ops.Equal(normalized, updated)
	assert.Always(equal,
		"Modern update semantics return requested object with server-assigned metadata when successful",
		params.Details(map[string]any{"name": current.GetMetadata().GetName()}),
	)

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Reading an updated resource returns the updated value",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, name)
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}

			return ops.Equal(updated, r), nil
		},
	})
}

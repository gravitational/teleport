package main

import (
	"context"
	"math/rand/v2"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/random"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/crud"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
)

func runReadYourWritesProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	created, err := ops.NewResource(fuzzerPrefix + "-" + uuid.New().String())
	if err != nil {
		return trace.Wrap(err, "creating new resource")
	}
	defer cleanupResource(ctx, adminOps, created)

	name := created.GetMetadata().GetName()
	details := params.Details(map[string]any{"name": name})

	_, err = ops.Create(ctx, created)
	// Hint to the fuzzer that we want cases where this succeeds to continue with the assertion.
	assert.Sometimes(err == nil, "Sometimes writing a resource for read-your-writes succeeds", details)
	if err != nil {
		return trace.Wrap(err, "creating resource")
	}

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Reading a written resource returns created value",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, name)
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}
			return equalResourceAfterServerAssignments(ops, created, r), nil
		},
	})
}

func runReadYourUpdatesProperty(ctx context.Context, params *TestCaseParams) error {
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

	// Some resources perform a read before doing an update. We are not guaranteed to be hitting the same
	// auth instance and thus this may fail due to cache propagation.
	err = eventually.Assert(ctx, eventually.AssertParams{
		Message: "Sometimes updating a resource for read-your-updates succeeds",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			_, err = ops.Update(ctx, current)
			return err == nil, err
		},
		Assertion: assert.Sometimes,
	})
	if err != nil {
		return trace.Wrap(err, "updating resource")
	}

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Reading an updated resource returns updated value",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, name)
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}

			return equalResourceAfterServerAssignments(ops, current, r), nil
		},
	})
}

func runDeleteVisibilityProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	resource, err := setupResource(ctx, adminOps, map[string]string{valueLabel: "to_be_deleted"})
	if err != nil {
		return trace.Wrap(err, "setting up resource")
	}
	defer cleanupResource(ctx, adminOps, resource)

	name := resource.GetMetadata().GetName()
	details := params.Details(map[string]any{"name": name})

	err = ops.Delete(ctx, name)
	assert.Sometimes(err == nil, "Sometimes deleting a resource succeeds", details)
	if err != nil {
		return trace.Wrap(err, "deleting resource")
	}

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Reading a deleted resource fails with not found",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			_, err := ops.Get153(ctx, name)
			if trace.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}
			return false, nil
		},
	})
}

func runUpdatesBumpRevisionProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	resource, err := setupResource(ctx, adminOps, map[string]string{valueLabel: "after-" + uuid.New().String()})
	if err != nil {
		return trace.Wrap(err, "setting up resource")
	}
	defer cleanupResource(ctx, adminOps, resource)

	name := resource.GetMetadata().GetName()
	revBefore := resource.GetMetadata().GetRevision()
	valueBefore := resource.GetMetadata().GetLabels()[valueLabel]
	valueWanted := "after-" + uuid.New().String()
	details := params.Details(map[string]any{
		"name":         name,
		"before_rev":   revBefore,
		"before_value": valueBefore,
		"want_value":   valueWanted,
	})

	if err := crud.SetStaticLabel(resource, valueLabel, valueWanted); err != nil {
		return trace.Wrap(err, "setting label")
	}

	// Some resources perform a read before doing an update. We are not guaranteed to be hitting the same
	// auth instance and thus this may fail due to cache propagation.
	err = eventually.Assert(ctx, eventually.AssertParams{
		Message: "Sometimes updating a resource with the current revision succeeds",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			_, err = ops.Update(ctx, resource)
			return err == nil, err
		},
		Assertion: assert.Sometimes,
	})
	if err != nil {
		return trace.Wrap(err, "updating resource")
	}

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Updating with the current revision advances the revision",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, name)
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}
			revAfter := r.GetMetadata().Revision
			valueAfter := r.GetMetadata().GetLabels()[valueLabel]
			addDetail("after_rev", revAfter)
			addDetail("after_value", valueAfter)
			return revBefore != "" &&
					revAfter != "" &&
					revBefore != revAfter &&
					valueAfter == valueWanted,
				nil
		},
	})
}

func runStaleResourceUpdateProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	resource, err := setupResource(ctx, adminOps, nil)
	if err != nil {
		return trace.Wrap(err, "setting up resource")
	}
	defer cleanupResource(ctx, adminOps, resource)

	name := resource.GetMetadata().GetName()
	details := params.Details(map[string]any{"name": name})

	currentValue := "current-" + uuid.New().String()
	if err := crud.SetStaticLabel(resource, valueLabel, currentValue); err != nil {
		return trace.Wrap(err, "setting label")
	}

	// Some resources perform a read before doing an update. We are not guaranteed to be hitting the same
	// auth instance and thus this may fail due to cache propagation.
	err = eventually.Assert(ctx, eventually.AssertParams{
		Message: "Sometimes updating a resource before stale-revision update succeeds",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			_, err = ops.Update(ctx, resource)
			return err == nil, err
		},
		Assertion: assert.Sometimes,
	})
	if err != nil {
		return trace.Wrap(err, "updating resource")
	}

	staleValue := "stale-" + uuid.New().String()
	if err := crud.SetStaticLabel(resource, valueLabel, staleValue); err != nil {
		return trace.Wrap(err, "setting label")
	}

	err = eventually.Assert(ctx, eventually.AssertParams{
		Message: "Updating with a stale revision is rejected",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
			_, err := ops.Update(ctx, resource)
			if trace.IsCompareFailed(err) {
				return true, nil
			} else if err == nil {
				assert.Unreachable("Updating resource with a stale revision should never succeed", details)
				return false, trace.Errorf("updating resource with a stale revision unexpectedly succeeded")
			}
			return false, err
		},
	})

	if err != nil {
		return trace.Wrap(err, "updating resource")
	}

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Stale revision update does not overwrite current value",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, name)
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}
			valueAfter := r.GetMetadata().GetLabels()[valueLabel]
			addDetail("got_value", valueAfter)
			return valueAfter == currentValue, nil
		},
	})
}

func runListExclusionAfterDeleteProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	resource, err := setupResource(ctx, adminOps, map[string]string{valueLabel: "to_be_deleted"})
	if err != nil {
		return trace.Wrap(err, "setting up resource")
	}
	defer cleanupResource(ctx, adminOps, resource)
	name := resource.GetMetadata().GetName()
	details := params.Details(map[string]any{"name": name})

	err = ops.Delete(ctx, name)
	assert.Sometimes(err == nil, "Sometimes deleting a resource before list succeeds", details)
	if err != nil {
		return trace.Wrap(err, "deleting resource")
	}

	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Deleted resource is absent from list",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			for r, err := range clientutils.Resources(ctx, ops.List153) {
				if err != nil {
					return false, trace.Wrap(err, "listing resources")
				}
				if r.GetMetadata().GetName() == name {
					return false, nil
				}

			}
			return true, nil
		},
	})
}

func runPaginationUniqueItemsProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	rng := rand.New(random.Source().(rand.Source))

	for range rng.Int32N(64) {
		resource, err := adminOps.NewResource(fuzzerPrefix + "-" + uuid.New().String())
		if err != nil {
			return trace.Wrap(err, "creating resource")
		}

		if _, err = ops.Create(ctx, resource); err != nil {
			return trace.Wrap(err, "creating fixture")
		}

		defer cleanupResource(ctx, adminOps, resource)
		// Note this test does not wait for resources to be created, in fact we hope some
		// may appear during the pagination.
	}

	pageSize := int(rng.Int32N(64))
	details := params.Details(map[string]any{"page_size": pageSize})

	if err := eventually.Assert(ctx, eventually.AssertParams{
		Message: "List succeeds before duplicate check",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			for _, err := range clientutils.ResourcesWithPageSize(ctx, ops.List153, pageSize) {
				if err != nil {
					return false, trace.Wrap(err, "listing resources")
				}
			}
			return true, nil
		},
	}); err != nil {
		return trace.Wrap(err, "waiting for list to succeed")
	}

	namemap := map[string]int{}
	for r, err := range clientutils.ResourcesWithPageSize(ctx, ops.List153, pageSize) {
		if err != nil {
			return trace.Wrap(err, "listing resources for duplicate check")
		}
		name := r.GetMetadata().GetName()
		namemap[name]++
		if namemap[name] > 1 {
			details["duplicate"] = name
			assert.AlwaysOrUnreachable(false, "List observes at most one copy of a resource", details)
			return trace.Errorf("list returned duplicate resource %q", name)
		}
	}

	assert.AlwaysOrUnreachable(true, "List observes at most one copy of a resource", details)
	return nil
}

func runConcurrentUpdateProperty(ctx context.Context, params *TestCaseParams) error {
	adminOps, ops, cleanup, err := setupOps(ctx, params)
	if err != nil {
		return trace.Wrap(err, "setting up resource ops")
	}
	defer cleanup(ctx)

	resource, err := setupResource(ctx, adminOps, nil)
	if err != nil {
		return trace.Wrap(err, "setting up resource")
	}
	defer cleanupResource(ctx, adminOps, resource)

	name := resource.GetMetadata().GetName()
	details := params.Details(map[string]any{"name": name})
	first := resource
	second := ops.Clone153(first)

	firstValue := "first-" + uuid.New().String()
	if err := crud.SetStaticLabel(first, valueLabel, firstValue); err != nil {
		return trace.Wrap(err, "setting label")
	}

	secondValue := "second-" + uuid.New().String()
	if err := crud.SetStaticLabel(second, valueLabel, secondValue); err != nil {
		return trace.Wrap(err, "setting label")
	}

	start := make(chan struct{})
	var firstErr, secondErr error
	var eg errgroup.Group
	eg.Go(func() error {
		<-start
		_, firstErr = ops.Update(ctx, first)
		return nil
	})
	eg.Go(func() error {
		<-start
		_, secondErr = ops.Update(ctx, second)
		return nil
	})
	close(start)
	_ = eg.Wait()

	successes, conflicts, indeterminate := 0, 0, 0
	for _, err := range []error{firstErr, secondErr} {
		switch {
		case err == nil:
			successes++
		case trace.IsCompareFailed(err):
			conflicts++
		default:
			indeterminate++
		}
	}
	details["first_error"] = firstErr
	details["second_error"] = secondErr
	details["successes"] = successes
	details["conflicts"] = conflicts
	details["indeterminate"] = indeterminate
	assert.Sometimes(successes == 1 && conflicts == 1, "CRUD concurrent race exercised", details)

	if indeterminate > 0 {
		return trace.Wrap(trace.NewAggregate(firstErr, secondErr), "concurrent updates returned indeterminate errors")
	}

	assert.AlwaysOrUnreachable(successes == 1 && conflicts == 1, "CRUD Concurrent updates to the same revision have one winner", details)
	if successes != 1 || conflicts != 1 {
		return trace.Errorf("concurrent updates unexpected outcome")
	}

	winnerValue := firstValue
	if secondErr == nil {
		winnerValue = secondValue
	}
	return eventually.Assert(ctx, eventually.AssertParams{
		Message: "Concurrent winner value is visible",
		Timeout: writeReplicationTimeout,
		Details: details,
		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
			r, err := ops.Get153(ctx, name)
			if err != nil {
				return false, trace.Wrap(err, "reading resource")
			}
			valueAfter := r.GetMetadata().GetLabels()[valueLabel]
			addDetail("got", valueAfter)
			addDetail("expected", winnerValue)
			return valueAfter == winnerValue, nil
		},
	})
}

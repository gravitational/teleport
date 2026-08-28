package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/random"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/crud"
	"github.com/gravitational/teleport/lib/utils"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
)

// Identities used in the CRUD fuzzer workload. These identities are expected to be
// present in the Teleport cluster and have the appropriate RBAC permissions for the
// workload to run successfully.
const (
	adminIdentity      = "admin"
	allowedIdentity    = "allowed"
	crudDeniedIdentity = "crud-denied"
)

const (
	fuzzerPrefix            = "antithesis"
	valueLabel              = "antithesis/value"
	writeReplicationTimeout = 5 * time.Minute
)

var testCases = []TestCase{

	{
		Name:     "read-your-writes: created resource eventually returns the same value when read",
		Kinds:    crud.DefaultKinds(),
		Identity: allowedIdentity,
		run:      runReadYourWritesProperty,
	},
	{
		Name:     "read-your-writes: modern create semantics return updated object",
		Kinds:    crud.DefaultKindsWithModernUpdateSemantics(),
		Identity: allowedIdentity,
		run:      runReadYourWritesModernProperty,
	},
	{
		Name:     "read-your-updates: updated resources are visible on subsequent reads",
		Kinds:    crud.DefaultKinds(),
		Identity: allowedIdentity,
		run:      runReadYourUpdatesProperty,
	},
	{
		Name:     "read-your-updates: modern update semantics return updated object",
		Kinds:    crud.DefaultKindsWithModernUpdateSemantics(),
		Identity: allowedIdentity,
		run:      runReadYourUpdatesModernProperty,
	},
	{
		Name:     "read-your-deletes: deleted resources are not readable",
		Kinds:    crud.DefaultKinds(),
		Identity: allowedIdentity,
		run:      runDeleteVisibilityProperty,
	},
	{
		Name:     "updates-change-revision: updates with the current revision succeed and advance the revision",
		Kinds:    crud.DefaultKinds(),
		Identity: allowedIdentity,
		run:      runUpdatesBumpRevisionProperty,
	},
	{
		Name:     "updates-respect-revision: updates with a stale revision are rejected",
		Kinds:    crud.DefaultKindsWithConditionalUpdate(),
		Identity: allowedIdentity,
		run:      runStaleResourceUpdateProperty,
	},
	{
		Name:     "collection-deletes-observed: deleted resources are excluded from subsequent lists",
		Kinds:    crud.DefaultKinds(),
		Identity: allowedIdentity,
		run:      runListExclusionAfterDeleteProperty,
	},
	{
		Name:     "collection-uniqueness: listing never observes duplicate resources",
		Kinds:    crud.DefaultKinds(),
		Identity: allowedIdentity,
		run:      runPaginationUniqueItemsProperty,
	},
	{
		Name:     "concurrent-update: concurrent updates to a single resource have 1 winner",
		Kinds:    crud.DefaultKindsWithConditionalUpdate(),
		Identity: allowedIdentity,
		run:      runConcurrentUpdateProperty,
	},
	{
		Name:     "deny: existing resource cannot be read",
		Kinds:    crud.DefaultKinds(),
		Identity: crudDeniedIdentity,
		run:      runDenyReadProperty,
	},
	{
		Name:     "allow: existing resource can be read",
		Kinds:    crud.DefaultKinds(),
		Identity: allowedIdentity,
		run:      runReadOnlyReadProperty,
	},
}

func main() {
	level := slog.LevelDebug
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level})))

	ctx, cancel := stacksignal.GetSignalHandler().NotifyContext(context.Background())
	defer cancel()

	if err := run(ctx, os.Getenv("ANTITHESIS_STOP_FAULTS") != ""); err != nil {
		fmt.Fprint(os.Stdout, utils.UserMessageFromError(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, runningInAntithesis bool) error {

	// When running in Antithesis, we only want to run a single property,
	// otherwise when running locally run every property to verify the harness itself.
	if runningInAntithesis {
		// We pick a random property group, then a random property for that group, and then a random kind for that property.
		// If for some reason, there is an order of operations which would result in a failure the fuzzer may be able to catch it.
		testcase := random.RandomChoice(testCases)

		params := &TestCaseParams{
			Kind:     random.RandomChoice(testcase.Kinds),
			Identity: testcase.Identity,
		}
		err := testcase.Run(ctx, params)

		details := map[string]any{
			"error":    err,
			"identity": params.Identity,
			"testcase": testcase.Name,
			"kind":     params.Kind,
		}
		assert.Sometimes(err == nil, "sometimes running property succeeds", details)
		if err != nil {
			// When running within antithesis, it is possible that certain properties will return an error
			// when under fault injection. We do not forward this as that is expected. But we log it for visibility.
			slog.WarnContext(ctx, "property failed",
				slog.String("error", err.Error()),
				slog.String("identity", params.Identity),
				slog.String("testcase", testcase.Name),
				slog.String("kind", params.Kind),
			)
		}
		return nil
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(1)

	for _, tc := range testCases {
		for _, kind := range tc.Kinds {
			eg.Go(func() error {
				return tc.Run(ctx, &TestCaseParams{
					Kind:     kind,
					Identity: tc.Identity,
				})
			})
		}
	}

	return eg.Wait()
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/random"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/utils"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
)

// Identities used in the SSH command workload. These identities are expected to be
// present in the Teleport cluster and have the appropriate RBAC permissions for the
// workload to run successfully.
const (
	adminIdentity   = "admin"
	allowedIdentity = "allowed"
	deniedIdentity  = "denied"
)

func main() {
	level := slog.LevelDebug
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level})))

	if err := run(context.Background(), os.Getenv("ANTITHESIS_STOP_FAULTS") != ""); err != nil {
		fmt.Fprint(os.Stdout, utils.UserMessageFromError(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, runningInAntithesis bool) error {
	ctx, cancel := stacksignal.GetSignalHandler().NotifyContext(ctx)
	defer cancel()

	testCases := []TestCase{
		{
			Name:     "allow: SSH command via hostname returns expected output and emits expected events.",
			run:      runAllowedSSHCommandProperty,
			Identity: allowedIdentity,
			Target: TargetSelector{
				Host: "agent",
			},
		},
		{
			Name:     "allow: SSH command via label returns expected output and emits expected events.",
			run:      runAllowedSSHCommandProperty,
			Identity: allowedIdentity,
			Target: TargetSelector{
				Labels: map[string]string{
					"test_template": "ssh",
				},
			},
		},
		{
			Name:     "allow: SSH command via predicate expression returns expected output and emits expected events.",
			run:      runAllowedSSHCommandProperty,
			Identity: allowedIdentity,
			Target: TargetSelector{
				PredicateExpression: `labels["test_template"] == "ssh"`,
			},
		},
		{
			Name:     "deny: SSH command via hostname is rejected.",
			run:      runDeniedSSHCommandProperty,
			Identity: deniedIdentity,
			Target: TargetSelector{
				Host: "agent",
			},
		},
		{
			Name:     "deny: SSH command via label is rejected.",
			run:      runDeniedSSHCommandProperty,
			Identity: deniedIdentity,
			Target: TargetSelector{
				Labels: map[string]string{
					"test_template": "ssh",
				},
			},
		},
		{
			Name:     "deny: SSH command via predicate expression is rejected.",
			run:      runDeniedSSHCommandProperty,
			Identity: deniedIdentity,
			Target: TargetSelector{
				PredicateExpression: `labels["test_template"] == "ssh"`,
			},
		},
	}

	// When running in Antithesis, we only want to run a single property,
	// otherwise when running locally run every property to verify the harness itself.
	if runningInAntithesis {
		testcase := random.RandomChoice(testCases)
		params := testcase.Params()

		err := testcase.Run(ctx, params)

		details := params.Details(map[string]any{
			"error":    err,
			"testcase": testcase.Name,
		})
		assert.Sometimes(err == nil, "sometimes running property succeeds", details)
		if err != nil {
			// when under fault injection. We do not forward this as that is expected. But we log it for visibility.
			slog.WarnContext(ctx, "property failed",
				slog.String("error", err.Error()),
				slog.String("identity", params.Identity),
				slog.String("testcase", testcase.Name),
			)
		}
		return nil
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(1)

	for _, tc := range testCases {
		eg.Go(func() error {
			return tc.Run(ctx, tc.Params())
		})
	}

	return eg.Wait()
}

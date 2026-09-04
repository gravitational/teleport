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
	const (
		dbAdminIdentity  = "allowed"
		dbDeniedIdentity = "denied"
	)

	testCases := []TestCase{
		{
			Name:     "allow: PostgreSQL query via static database name returns expected rows and emits expected events.",
			run:      runAllowedDatabaseAccessProperty,
			Identity: dbAdminIdentity,
			Target: DatabaseTarget{
				Name: "static",
			},
		},
		{
			Name:     "allow: PostgreSQL query via dynamic database name returns expected rows and emits expected events.",
			run:      runAllowedDatabaseAccessProperty,
			Identity: dbAdminIdentity,
			Target: DatabaseTarget{
				Name: "dynamic",
			},
		},
		{
			Name:     "allow: PostgreSQL query via static database labels returns expected rows and emits expected events.",
			run:      runAllowedDatabaseAccessProperty,
			Identity: dbAdminIdentity,
			Target: DatabaseTarget{
				Labels: map[string]string{
					"test_template": "db",
					"type":          "static",
				},
			},
		},
		{
			Name:     "allow: PostgreSQL query via dynamic database labels returns expected rows and emits expected events.",
			run:      runAllowedDatabaseAccessProperty,
			Identity: dbAdminIdentity,
			Target: DatabaseTarget{
				Labels: map[string]string{
					"test_template": "db",
					"type":          "dynamic",
				},
			},
		},
		{
			Name:     "allow: PostgreSQL query via static database predicate expression returns expected rows and emits expected events.",
			run:      runAllowedDatabaseAccessProperty,
			Identity: dbAdminIdentity,
			Target: DatabaseTarget{
				PredicateExpression: `labels["test_template"] == "db" && labels["type"] == "static"`,
			},
		},
		{
			Name:     "allow: PostgreSQL query via dynamic database predicate expression returns expected rows and emits expected events.",
			run:      runAllowedDatabaseAccessProperty,
			Identity: dbAdminIdentity,
			Target: DatabaseTarget{
				PredicateExpression: `labels["test_template"] == "db" && labels["type"] == "dynamic"`,
			},
		},
		{
			Name:     "deny: PostgreSQL query via static database name is rejected.",
			run:      runDeniedDatabaseAccessProperty,
			Identity: dbDeniedIdentity,
			Target: DatabaseTarget{
				Name: "static",
			},
		},
		{
			Name:     "deny: PostgreSQL query via dynamic database name is rejected.",
			run:      runDeniedDatabaseAccessProperty,
			Identity: dbDeniedIdentity,
			Target: DatabaseTarget{
				Name: "dynamic",
			},
		},
		{
			Name:     "deny: static database is not resolvable by name.",
			run:      runDeniedDatabaseResolutionProperty,
			Identity: dbDeniedIdentity,
			Target: DatabaseTarget{
				Name: "static",
			},
		},
		{
			Name:     "deny: static database is not resolvable by labels.",
			run:      runDeniedDatabaseResolutionProperty,
			Identity: dbDeniedIdentity,
			Target: DatabaseTarget{
				Labels: map[string]string{
					"test_template": "db",
					"type":          "static",
				},
			},
		},
		{
			Name:     "deny: dynamic database is not resolvable by predicate expression.",
			run:      runDeniedDatabaseResolutionProperty,
			Identity: dbDeniedIdentity,
			Target: DatabaseTarget{
				PredicateExpression: `labels["test_template"] == "db" && labels["type"] == "dynamic"`,
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
		assert.Sometimes(err == nil, "A database access driver run can complete successfully", details)
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

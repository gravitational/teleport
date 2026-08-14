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

	"github.com/gravitational/teleport/lib/utils"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
)

const (
	adminIdentity     = "admin"
	appAdminIdentity  = "app-admin"
	appMintIdentity   = "app-mint"
	appDeniedIdentity = "app-denied"
	botUsername       = "bot-workload"
)

const (
	defaultClusterName      = "antithesis.teleport.local"
	auditEventEmitDeadline  = 20 * time.Minute
	auditSessionChunkTTL    = 5 * time.Minute
	maxTolerableClockJitter = 10 * time.Minute
)

const (
	appExpectedBody = "antithesis-app-ok"
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
			Name:     "allow: HTTP app access to alpha with tbot credentials succeeds and emits expected events.",
			run:      runAllowedAppAccessTbotCredProperty,
			Identity: appAdminIdentity,
			App: AppTarget{
				Name:       "alpha",
				PublicAddr: "app-alpha.antithesis.teleport.local",
				URI:        "http://app-alpha:80",
			},
		},
		{
			Name:     "allow: HTTP app access to alpha with minted credentials succeeds and emits session start.",
			run:      runAllowedAppAccessMintedCredProperty,
			Identity: appMintIdentity,
			App: AppTarget{
				Name:       "alpha",
				PublicAddr: "app-alpha.antithesis.teleport.local",
				URI:        "http://app-alpha:80",
			},
		},
		{
			Name:     "allow: TCP app access to alpha-tcp with minted credentials succeeds and emits expected events.",
			run:      runAllowedTCPAppAccessMintedCredProperty,
			Identity: appMintIdentity,
			App: AppTarget{
				Name:       "alpha-tcp",
				PublicAddr: "app-alpha-tcp.antithesis.teleport.local",
				URI:        "tcp://app-alpha:80",
			},
		},
		{
			Name:     "deny: alpha app access with minted credentials is rejected.",
			run:      runDeniedAppAccessCredentialMintProperty,
			Identity: appDeniedIdentity,
			App: AppTarget{
				Name:       "alpha",
				PublicAddr: "app-alpha.antithesis.teleport.local",
				URI:        "http://app-alpha:80",
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
		assert.Sometimes(err == nil, "An app access driver run can complete successfully", details)
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

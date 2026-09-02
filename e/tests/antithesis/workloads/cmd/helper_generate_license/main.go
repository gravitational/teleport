package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/license/generate"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func main() {
	ctx := context.Background()
	level := slog.LevelInfo
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &level})))

	if err := Run(ctx, os.Args[1:]); err != nil {
		slog.ErrorContext(ctx, "failed to generate license", "error", trace.DebugReport(err))
		os.Exit(1)
	}
}

func Run(ctx context.Context, args []string) error {
	var outPath string
	app := kingpin.New("generate-license", "Generate a valid Teleport license for testing.").Interspersed(true)
	app.Flag("out", "path to output license").Default("./license.pem").Envar("LICENSE_FILE").StringVar(&outPath)

	if _, err := app.Parse(args); err != nil {
		app.Usage(args)
		return trace.Wrap(err, "parsing args")
	}

	return runGeneration(ctx, outPath)
}

func runGeneration(ctx context.Context, outPath string) error {
	const validFor = 3 * 24 * time.Hour

	keyPair, err := authority.GenerateSelfSignedCA("antithesis.teleport.local", validFor)
	if err != nil {
		return trace.Wrap(err, "generating license CA")
	}

	spec := types.LicenseSpecV3{
		AccountID:         uuid.NewString(),
		Cloud:             types.NewBool(false),
		Trial:             false,
		UsageBasedBilling: types.NewBool(false),

		SupportsKubernetes:                 true,
		SupportsDatabaseAccess:             true,
		SupportsDesktopAccess:              true,
		SupportsModeratedSessions:          true,
		SupportsMachineID:                  true,
		SupportsResourceAccessRequests:     true,
		SupportsFeatureHiding:              true,
		SupportsIdentityGovernanceSecurity: true,
		SupportsPolicy:                     true,
	}

	license, err := types.NewLicense(uuid.NewString(), spec)
	if err != nil {
		return trace.Wrap(err, "creating license")
	}

	license.SetExpiry(time.Now().UTC().Add(validFor))

	payload, err := json.Marshal(license)
	if err != nil {
		return trace.Wrap(err, "marshaling license")
	}

	privateKey, err := generate.NewPrivateKey()
	if err != nil {
		return trace.Wrap(err)
	}

	pem, err := generate.NewLicense(generate.NewLicenseInfo{
		ValidFor:   validFor,
		Payload:    payload,
		TLSKeyPair: *keyPair,
		PrivateKey: privateKey,
	})
	if err != nil {
		return trace.Wrap(err, "generating license")
	}

	if err = os.WriteFile(outPath, []byte(pem), 0600); err != nil {
		return trace.Wrap(err, "writing license")
	}

	slog.InfoContext(ctx, "generated license",
		slog.String("path", outPath),
		slog.String("valid_for", validFor.String()))
	return nil
}

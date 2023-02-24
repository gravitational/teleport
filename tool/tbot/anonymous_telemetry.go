package main

import (
	"context"
	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha/v1alphaconnect"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	anonymousTelemetryEnabledEnv  = "TELEPORT_ANONYMOUS_TELEMETRY"
	anonymousTelemetryEndpointEnv = "TELEPORT_ANONYMOUS_TELEMETRY_ENDPOINT"

	helperEnv        = "_TBOT_TELEMETRY_HELPER"
	helperVersionEnv = "_TBOT_TELEMETRY_HELPER_VERSION"
)

func telemetryEndpoint() string {
	env := os.Getenv(anonymousTelemetryEndpointEnv)
	if env != "" {
		return env
	}

	// staging: https://reporting-staging.teleportinfra.dev
	return "https://reporting.teleportinfra.sh"
}

func telemetryEnabled() bool {
	if val, err := strconv.ParseBool(
		os.Getenv(anonymousTelemetryEnabledEnv),
	); err == nil {
		return val
	}
	return false
}

// sendTelemetry sends the anonymous on start Telemetry event.
// It is imperative that this code does not send any user or teleport instance
// identifiable information.
func sendTelemetry(ctx context.Context, log logrus.FieldLogger, cfg *config.BotConfig) error {
	start := time.Now()
	if !telemetryEnabled() {
		// TODO(noah): Have this not be a placeholder message
		log.Warn("no telemetry placeholder message")
		return nil
	}
	log.Warn("telemetry enabled placeholder message")

	data := &v1alpha.TbotStartEvent{
		// TODO: Determine run mode
		RunMode: v1alpha.TbotStartEvent_RUN_MODE_UNSPECIFIED,
		// Default to reporting the "token" join method to account for
		// scenarios where initial join has onboarding configured but future
		// starts renew using credentials.
		JoinType: string(types.JoinMethodToken),
		Version:  teleport.Version,
	}
	if helper := os.Getenv(helperEnv); helper != "" {
		data.Helper = helper
		data.HelperVersion = os.Getenv(helperVersionEnv)
	}
	if cfg.Onboarding != nil && cfg.Onboarding.JoinMethod != "" {
		data.JoinType = string(cfg.Onboarding.JoinMethod)
	}
	for _, dest := range cfg.Destinations {
		switch {
		case dest.App != nil:
			data.DestinationsApplication++
		case dest.Database != nil:
			data.DestinationsDatabase++
		case dest.KubernetesCluster != nil:
			data.DestinationsKubernetes++
		default:
			data.DestinationsOther++
		}
	}

	client := v1alphaconnect.NewTbotReportingServiceClient(
		http.DefaultClient, telemetryEndpoint(),
	)
	distinctID := uuid.New().String()
	_, err := client.SubmitTbotEvent(ctx, connect.NewRequest(&v1alpha.SubmitTbotEventRequest{
		DistinctId: distinctID,
		Timestamp:  timestamppb.Now(),
		Event:      &v1alpha.SubmitTbotEventRequest_Start{Start: data},
	}))
	if err != nil {
		return trace.Wrap(err)
	}
	log.WithField("distinct_id", distinctID).
		WithField("duration", time.Since(start)).
		Debug("Successfully transmitted anonymous telemetry")

	return nil
}

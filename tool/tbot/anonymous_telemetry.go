// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	prehogv1ac "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha/prehogv1alphaconnect"
	"github.com/gravitational/teleport/lib/tbot/config"
)

const (
	anonymousTelemetryEnabledEnv = "TELEPORT_ANONYMOUS_TELEMETRY"
	anonymousTelemetryAddressEnv = "TELEPORT_ANONYMOUS_TELEMETRY_ADDRESS"

	helperEnv        = "_TBOT_TELEMETRY_HELPER"
	helperVersionEnv = "_TBOT_TELEMETRY_HELPER_VERSION"

	telemetryDocs = "https://goteleport.com/docs/machine-id/reference/telemetry/"
)

type envGetter func(key string) string

func telemetryEnabled(envGetter envGetter) bool {
	if val, err := strconv.ParseBool(
		envGetter(anonymousTelemetryEnabledEnv),
	); err == nil {
		return val
	}
	return false
}

func telemetryClient(envGetter envGetter) prehogv1ac.TbotReportingServiceClient {
	// Override the default value using TELEPORT_ANONYMOUS_TELEMETRY_ADDRESS
	// environment variable.
	// staging: https://reporting-tbot-staging.teleportinfra.dev
	endpoint := "https://reporting-tbot.teleportinfra.sh"
	if env := envGetter(anonymousTelemetryAddressEnv); env != "" {
		endpoint = env
	}

	return prehogv1ac.NewTbotReportingServiceClient(
		http.DefaultClient, endpoint,
	)
}

// sendTelemetry sends the anonymous on start Telemetry event.
// It is imperative that this code does not send any user or teleport instance
// identifiable information.
func sendTelemetry(
	ctx context.Context,
	client prehogv1ac.TbotReportingServiceClient,
	envGetter envGetter,
	log logrus.FieldLogger,
	cfg *config.BotConfig,
) error {
	start := time.Now()
	if !telemetryEnabled(envGetter) {
		log.Infof("Anonymous telemetry is not enabled. Find out more about Machine ID's anonymous telemetry at %s", telemetryDocs)
		return nil
	}
	log.Infof("Anonymous telemetry is enabled. Find out more about Machine ID's anonymous telemetry at %s", telemetryDocs)

	data := &prehogv1a.TbotStartEvent{
		RunMode: prehogv1a.TbotStartEvent_RUN_MODE_DAEMON,
		// Default to reporting the "token" join method to account for
		// scenarios where initial join has onboarding configured but future
		// starts renew using credentials.
		JoinType: string(types.JoinMethodToken),
		Version:  teleport.Version,
	}
	if cfg.Oneshot {
		data.RunMode = prehogv1a.TbotStartEvent_RUN_MODE_ONE_SHOT
	}
	if helper := envGetter(helperEnv); helper != "" {
		data.Helper = helper
		data.HelperVersion = envGetter(helperVersionEnv)
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

	distinctID := uuid.New().String()
	_, err := client.SubmitTbotEvent(ctx, connect.NewRequest(&prehogv1a.SubmitTbotEventRequest{
		DistinctId: distinctID,
		Timestamp:  timestamppb.New(start),
		Event:      &prehogv1a.SubmitTbotEventRequest_Start{Start: data},
	}))
	if err != nil {
		return trace.Wrap(err)
	}
	log.WithField("distinct_id", distinctID).
		WithField("duration", time.Since(start)).
		Debug("Successfully transmitted anonymous telemetry")

	return nil
}

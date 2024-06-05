/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	prehogv1ac "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha/prehogv1alphaconnect"
	"github.com/gravitational/teleport/lib/tbot/config"
)

const (
	anonymousTelemetryEnabledEnv = "TELEPORT_ANONYMOUS_TELEMETRY"
	anonymousTelemetryAddressEnv = "TELEPORT_ANONYMOUS_TELEMETRY_ADDRESS"

	helperEnv        = "_TBOT_TELEMETRY_HELPER"
	helperVersionEnv = "_TBOT_TELEMETRY_HELPER_VERSION"
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
	log *slog.Logger,
	cfg *config.BotConfig,
) error {
	start := time.Now()
	if !telemetryEnabled(envGetter) {
		log.InfoContext(ctx, "Anonymous telemetry is not enabled. Find out more about Machine ID's anonymous telemetry at https://goteleport.com/docs/machine-id/reference/telemetry/")
		return nil
	}
	log.InfoContext(ctx, "Anonymous telemetry is enabled. Find out more about Machine ID's anonymous telemetry at https://goteleport.com/docs/machine-id/reference/telemetry/")

	data := &prehogv1a.TbotStartEvent{
		RunMode:  prehogv1a.TbotStartEvent_RUN_MODE_DAEMON,
		JoinType: string(cfg.Onboarding.JoinMethod),
		Version:  teleport.Version,
	}
	if cfg.Oneshot {
		data.RunMode = prehogv1a.TbotStartEvent_RUN_MODE_ONE_SHOT
	}
	if helper := envGetter(helperEnv); helper != "" {
		data.Helper = helper
		data.HelperVersion = envGetter(helperVersionEnv)
	}
	for _, output := range cfg.Outputs {
		switch output.(type) {
		case *config.ApplicationOutput:
			data.DestinationsApplication++
		case *config.DatabaseOutput:
			data.DestinationsDatabase++
		case *config.KubernetesOutput:
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
	log.DebugContext(
		ctx,
		"Successfully transmitted anonymous telemetry",
		"distinct_id", distinctID,
		"duration", time.Since(start),
	)

	return nil
}

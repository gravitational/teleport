/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/lib/tbot/config"
)

type heartbeatSubmitter interface {
	SubmitHeartbeat(
		ctx context.Context, in *machineidv1pb.SubmitHeartbeatRequest, opts ...grpc.CallOption,
	) (*machineidv1pb.SubmitHeartbeatResponse, error)
}

type heartbeatService struct {
	now                func() time.Time
	log                *slog.Logger
	botCfg             *config.BotConfig
	startedAt          time.Time
	heartbeatSubmitter heartbeatSubmitter
	botIdentityReadyCh <-chan struct{}
	interval           time.Duration
	retryLimit         int
}

func (s *heartbeatService) heartbeat(ctx context.Context, isStartup bool) error {
	s.log.DebugContext(ctx, "Sending heartbeat")
	hostName, err := os.Hostname()
	if err != nil {
		s.log.WarnContext(ctx, "Failed to determine hostname for heartbeat", "error", err)
	}

	hb := &machineidv1pb.BotInstanceStatusHeartbeat{
		RecordedAt:   timestamppb.New(s.now()),
		Hostname:     hostName,
		IsStartup:    isStartup,
		Uptime:       durationpb.New(s.now().Sub(s.startedAt)),
		OneShot:      s.botCfg.Oneshot,
		JoinMethod:   string(s.botCfg.Onboarding.JoinMethod),
		Version:      teleport.Version,
		Architecture: runtime.GOARCH,
		Os:           runtime.GOOS,
	}

	_, err = s.heartbeatSubmitter.SubmitHeartbeat(ctx, &machineidv1pb.SubmitHeartbeatRequest{
		Heartbeat: hb,
	})
	if err != nil {
		return trace.Wrap(err, "submitting heartbeat")
	}

	s.log.InfoContext(ctx, "Sent heartbeat", "data", hb.String())
	return nil
}

func (s *heartbeatService) OneShot(ctx context.Context) error {
	err := s.heartbeat(ctx, true)
	// Ignore not implemented as this is likely confusing.
	// TODO(noah): Remove NotImplemented check at V18 assuming V17 first major
	// with heartbeating.
	if err != nil && !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *heartbeatService) Run(ctx context.Context) error {
	isStartup := true
	err := runOnInterval(ctx, runOnIntervalConfig{
		service:    s.String(),
		name:       "submit-heartbeat",
		log:        s.log,
		interval:   s.interval,
		retryLimit: s.retryLimit,
		f: func(ctx context.Context) error {
			err := s.heartbeat(ctx, isStartup)
			// TODO(noah): Remove NotImplemented check at V18 assuming V17 first
			// major with heartbeating.
			if trace.IsNotImplemented(err) {
				s.log.DebugContext(
					ctx,
					"Cluster does not support Bot Instance heartbeats",
				)
				return nil
			}
			if err != nil {
				return trace.Wrap(err, "submitting heartbeat")
			}
			isStartup = false
			return nil
		},
		identityReadyCh: s.botIdentityReadyCh,
	})
	return trace.Wrap(err)
}

func (s *heartbeatService) String() string {
	return "heartbeat"
}

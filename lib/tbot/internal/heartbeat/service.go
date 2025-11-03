/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package heartbeat

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

const (
	// Maximum amount of time we'll wait for services to report their initial
	// status before sending the first heartbeat.
	serviceHealthMaxWait = 30 * time.Second

	// Maximum amount of time the one-shot heartbeat can take once the bot has
	// started shutting down.
	shutdownHeartbeatTimeout = 5 * time.Second
)

// Client for the heartbeat service.
type Client interface {
	SubmitHeartbeat(
		ctx context.Context, in *machineidv1pb.SubmitHeartbeatRequest, opts ...grpc.CallOption,
	) (*machineidv1pb.SubmitHeartbeatResponse, error)
}

// Config for the heartbeat service.
type Config struct {
	// BotKind identifies whether the bot is running in the tbot binary or
	// embedded in another component
	BotKind machineidv1pb.BotKind

	// Interval controls how frequently heartbeats are submitted.
	Interval time.Duration

	// RetryLimit is the maximum number of times we'll retry sending a heartbeat.
	RetryLimit int

	// Client that will be used to submit heartbeats.
	Client Client

	// Logger to which errors and messages will be written.
	Logger *slog.Logger

	// JoinMethod is the bot join method that will be reported.
	JoinMethod types.JoinMethod

	// StartedAt is the time at which the bot was started.
	StartedAt time.Time

	// BotIdentityReadyCh is a channel that the service will receive from to
	// block until the bot's internal identity is ready.
	BotIdentityReadyCh <-chan struct{}

	// StatusReporter is used to report the service's health.
	StatusReporter readyz.Reporter

	// StatusRegistry is used to fetch the current service statuses when
	// submitting a heartbeat.
	StatusRegistry *readyz.Registry
}

// CheckAndSetDefaults checks the service configuration and sets any default values.
func (cfg *Config) CheckAndSetDefaults() error {
	switch {
	case cfg.Interval == 0:
		return trace.BadParameter("Interval is required")
	case cfg.RetryLimit == 0:
		return trace.BadParameter("RetryLimit is required")
	case cfg.Client == nil:
		return trace.BadParameter("Client is required")
	case cfg.JoinMethod == "":
		return trace.BadParameter("JoinMethod is required")
	case cfg.StatusRegistry == nil:
		return trace.BadParameter("StatusRegistry is required")
	}
	if cfg.StartedAt.IsZero() {
		cfg.StartedAt = time.Now()
	}
	return nil
}

// NewService creates the heartbeat service.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{cfg: cfg}, nil
}

// Service implements bot heartbeating.
type Service struct{ cfg Config }

// Run the service in long-running mode, submitting heartbeats periodically.
func (s *Service) Run(ctx context.Context) error {
	// Wait for service health before sending our first heartbeat. Otherwise, we
	// might report all services as "initializing" for the first ~30 minutes our
	// bot is running.
	if shuttingDown := s.waitForServiceHealth(ctx); shuttingDown {
		return nil
	}

	isStartup := true
	err := internal.RunOnInterval(ctx, internal.RunOnIntervalConfig{
		Service:    s.String(),
		Name:       "submit-heartbeat",
		Log:        s.cfg.Logger,
		Interval:   s.cfg.Interval,
		RetryLimit: s.cfg.RetryLimit,
		F: func(ctx context.Context) error {
			err := s.heartbeat(ctx, false, isStartup)
			// TODO(noah): Remove NotImplemented check at V18 assuming V17 first
			// major with heartbeating.
			if trace.IsNotImplemented(err) {
				s.cfg.Logger.DebugContext(
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
		IdentityReadyCh: s.cfg.BotIdentityReadyCh,
		StatusReporter:  s.cfg.StatusReporter,
	})
	return trace.Wrap(err)
}

// OneShot submits one heartbeat and then exits.
func (s *Service) OneShot(ctx context.Context) error {
	// Wait for services to report their health before sending the heartbeat.
	shuttingDown := s.waitForServiceHealth(ctx)

	if shuttingDown {
		// If the outer context has been canceled (likely because another
		// service has return an error) we'll create a new one detached from
		// the cancellation to try to send the heartbeat.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			context.WithoutCancel(ctx),
			shutdownHeartbeatTimeout,
		)
		defer cancel()
	}

	err := s.heartbeat(ctx, true, true)
	// Ignore not implemented as this is likely confusing.
	// TODO(noah): Remove NotImplemented check at V18 assuming V17 first major
	// with heartbeating.
	if err != nil && !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}
	return nil
}

// String implements fmt.Stringer.
func (s *Service) String() string { return "heartbeat" }

func (s *Service) waitForServiceHealth(ctx context.Context) (shuttingDown bool) {
	// We must report our own status to avoid blocking ourselves!
	s.cfg.StatusReporter.Report(readyz.Healthy)

	select {
	case <-s.cfg.StatusRegistry.AllServicesReported():
		// All services have reported their status, we're ready!
		return false
	case <-time.After(serviceHealthMaxWait):
		// It's taking too long, give up and start sending heartbeats.
		return false
	case <-ctx.Done():
		// The outer context has been canceled (e.g. another service has exited
		// or the process has received SIGINT).
		return true
	}
}

func (s *Service) heartbeat(ctx context.Context, isOneShot, isStartup bool) error {
	s.cfg.Logger.DebugContext(ctx, "Sending heartbeat")
	hostName, err := os.Hostname()
	if err != nil {
		s.cfg.Logger.WarnContext(ctx, "Failed to determine hostname for heartbeat", "error", err)
	}

	now := time.Now()
	hb := &machineidv1pb.BotInstanceStatusHeartbeat{
		RecordedAt:   timestamppb.New(now),
		Hostname:     hostName,
		IsStartup:    isStartup,
		Uptime:       durationpb.New(now.Sub(s.cfg.StartedAt)),
		OneShot:      isOneShot,
		JoinMethod:   string(s.cfg.JoinMethod),
		Version:      teleport.Version,
		Architecture: runtime.GOARCH,
		Os:           runtime.GOOS,
		Kind:         s.cfg.BotKind,
	}

	status := s.cfg.StatusRegistry.OverallStatus()
	serviceHealth := make([]*machineidv1pb.BotInstanceServiceHealth, 0, len(status.Services))
	for name, serviceStatus := range status.Services {
		serviceHealth = append(serviceHealth, statusToServiceHealth(name, serviceStatus))
	}

	// Sort the services by name to make tests deterministic.
	sort.Slice(serviceHealth, func(a, b int) bool {
		return serviceHealth[a].Service.Name < serviceHealth[b].Service.Name
	})

	_, err = s.cfg.Client.SubmitHeartbeat(ctx, &machineidv1pb.SubmitHeartbeatRequest{
		Heartbeat:     hb,
		ServiceHealth: serviceHealth,
	})
	if err != nil {
		return trace.Wrap(err, "submitting heartbeat")
	}

	s.cfg.Logger.InfoContext(ctx, "Sent heartbeat", "data", hb.String())
	return nil
}

func statusToServiceHealth(name string, status *readyz.ServiceStatus) *machineidv1pb.BotInstanceServiceHealth {
	health := &machineidv1pb.BotInstanceServiceHealth{
		Service: &machineidv1pb.BotInstanceServiceIdentifier{
			Name: trimString(name, 64),
			Type: status.ServiceType,
		},
	}

	switch status.Status {
	case readyz.Initializing:
		health.Status = machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_INITIALIZING
	case readyz.Healthy:
		health.Status = machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_HEALTHY
	case readyz.Unhealthy:
		health.Status = machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
	}

	if status.Reason != "" {
		reason := trimString(status.Reason, 256)
		health.Reason = &reason
	}

	if status.UpdatedAt != nil {
		health.UpdatedAt = timestamppb.New(*status.UpdatedAt)
	}

	return health
}

func trimString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// Trim the string to maxBytes, honoring rune boundaries for non-ASCII text.
	byteCount := 0
	for i, r := range s {
		runeSize := len(string(r))
		if byteCount+runeSize > maxBytes {
			return s[:i]
		}
		byteCount += runeSize
	}
	return s
}

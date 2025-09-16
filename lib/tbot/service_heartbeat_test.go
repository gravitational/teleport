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
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

type fakeHeartbeatSubmitter struct {
	ch chan *machineidv1pb.SubmitHeartbeatRequest
}

func (f *fakeHeartbeatSubmitter) SubmitHeartbeat(
	ctx context.Context, in *machineidv1pb.SubmitHeartbeatRequest, opts ...grpc.CallOption,
) (*machineidv1pb.SubmitHeartbeatResponse, error) {
	f.ch <- in
	return &machineidv1pb.SubmitHeartbeatResponse{}, nil
}

func TestHeartbeatService(t *testing.T) {
	t.Parallel()

	log := utils.NewSlogLoggerForTests()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fhs := &fakeHeartbeatSubmitter{
		ch: make(chan *machineidv1pb.SubmitHeartbeatRequest, 2),
	}
	svc := heartbeatService{
		now: func() time.Time {
			return time.Date(2024, time.April, 1, 12, 0, 0, 0, time.UTC)
		},
		log: log,
		botCfg: &config.BotConfig{
			Oneshot: false,
			Onboarding: config.OnboardingConfig{
				JoinMethod: types.JoinMethodGitHub,
			},
		},
		startedAt:          time.Date(2024, time.April, 1, 11, 0, 0, 0, time.UTC),
		heartbeatSubmitter: fhs,
		interval:           time.Second,
		retryLimit:         3,
	}

	hostName, err := os.Hostname()
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.Run(ctx)
	}()

	got := <-fhs.ch
	want := &machineidv1pb.SubmitHeartbeatRequest{
		Heartbeat: &machineidv1pb.BotInstanceStatusHeartbeat{
			RecordedAt:   timestamppb.New(svc.now()),
			Hostname:     hostName,
			IsStartup:    true,
			OneShot:      false,
			Uptime:       durationpb.New(time.Hour),
			Version:      teleport.Version,
			Architecture: runtime.GOARCH,
			Os:           runtime.GOOS,
			JoinMethod:   string(types.JoinMethodGitHub),
		},
	}
	assert.Empty(t, cmp.Diff(want, got, protocmp.Transform()))

	got = <-fhs.ch
	want.Heartbeat.IsStartup = false
	assert.Empty(t, cmp.Diff(want, got, protocmp.Transform()))

	cancel()
	assert.NoError(t, <-errCh)
}

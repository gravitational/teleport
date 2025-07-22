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
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
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

	log := logtest.NewLogger()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fhs := &fakeHeartbeatSubmitter{
		ch: make(chan *machineidv1pb.SubmitHeartbeatRequest, 2),
	}

	now := time.Date(2024, time.April, 1, 12, 0, 0, 0, time.UTC)
	svc, err := NewService(Config{
		Interval:   time.Second,
		RetryLimit: 3,
		Client:     fhs,
		Clock:      clockwork.NewFakeClockAt(now),
		StartedAt:  time.Date(2024, time.April, 1, 11, 0, 0, 0, time.UTC),
		Logger:     log,
		JoinMethod: types.JoinMethodGitHub,
	})
	require.NoError(t, err)

	hostName, err := os.Hostname()
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.Run(ctx)
	}()

	got := <-fhs.ch
	want := &machineidv1pb.SubmitHeartbeatRequest{
		Heartbeat: &machineidv1pb.BotInstanceStatusHeartbeat{
			RecordedAt:   timestamppb.New(now),
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

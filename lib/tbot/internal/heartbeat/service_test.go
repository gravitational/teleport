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
	"strings"
	"testing"
	"testing/synctest"
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
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

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

	synctest.Test(t, func(t *testing.T) {
		log := logtest.NewLogger()
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		fhs := &fakeHeartbeatSubmitter{
			ch: make(chan *machineidv1pb.SubmitHeartbeatRequest, 2),
		}

		reg := readyz.NewRegistry()

		svcA := reg.AddService("a", "a")
		svcB := reg.AddService("b", strings.Repeat("b", 200))

		svc, err := NewService(Config{
			Interval:       time.Second,
			RetryLimit:     3,
			Client:         fhs,
			StartedAt:      time.Now().Add(-1 * time.Hour),
			Logger:         log,
			JoinMethod:     types.JoinMethodGitHub,
			StatusReporter: reg.AddService("internal/heartbeat", "heartbeat"),
			StatusRegistry: reg,
			BotKind:        machineidv1pb.BotKind_BOT_KIND_TBOT,
		})
		require.NoError(t, err)

		hostName, err := os.Hostname()
		require.NoError(t, err)

		errCh := make(chan error, 1)
		go func() {
			errCh <- svc.Run(ctx)
		}()

		synctest.Wait()
		select {
		case <-fhs.ch:
			t.Fatal("should not have received a heartbeat until all services have reported their status")
		default:
		}

		svcA.ReportReason(readyz.Unhealthy, "no more bananas")
		svcB.ReportReason(readyz.Unhealthy, strings.Repeat("b", 300))

		want := machineidv1pb.SubmitHeartbeatRequest_builder{
			Heartbeat: machineidv1pb.BotInstanceStatusHeartbeat_builder{
				Hostname:     hostName,
				IsStartup:    true,
				OneShot:      false,
				Uptime:       durationpb.New(time.Hour),
				Version:      teleport.Version,
				Architecture: runtime.GOARCH,
				Os:           runtime.GOOS,
				JoinMethod:   string(types.JoinMethodGitHub),
				Kind:         machineidv1pb.BotKind_BOT_KIND_TBOT,
			}.Build(),
			ServiceHealth: []*machineidv1pb.BotInstanceServiceHealth{
				machineidv1pb.BotInstanceServiceHealth_builder{
					Service: machineidv1pb.BotInstanceServiceIdentifier_builder{
						Name: "a",
						Type: "a",
					}.Build(),
					Reason:    ptr("no more bananas"),
					Status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY,
					UpdatedAt: timestamppb.New(time.Now()),
				}.Build(),
				// Check limits were applied on user-controlled or dynamic fields.
				machineidv1pb.BotInstanceServiceHealth_builder{
					Service: machineidv1pb.BotInstanceServiceIdentifier_builder{
						Name: strings.Repeat("b", 64),
						Type: "b",
					}.Build(),
					Reason:    ptr(strings.Repeat("b", 256)),
					Status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY,
					UpdatedAt: timestamppb.New(time.Now()),
				}.Build(),
				machineidv1pb.BotInstanceServiceHealth_builder{
					Service: machineidv1pb.BotInstanceServiceIdentifier_builder{
						Name: "heartbeat",
						Type: "internal/heartbeat",
					}.Build(),
					Status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_HEALTHY,
					UpdatedAt: timestamppb.New(time.Now()),
				}.Build(),
			},
		}.Build()

		compare := func(t *testing.T, want, got *machineidv1pb.SubmitHeartbeatRequest) {
			t.Helper()

			assert.Empty(t,
				cmp.Diff(want, got,
					protocmp.Transform(),
					protocmp.IgnoreFields(&machineidv1pb.BotInstanceStatusHeartbeat{}, "recorded_at"),
				),
			)
		}

		synctest.Wait()
		select {
		case got := <-fhs.ch:
			compare(t, want, got)
		default:
			t.Fatal("no heartbeat received")
		}

		time.Sleep(1 * time.Second)
		synctest.Wait()

		select {
		case got := <-fhs.ch:
			want.GetHeartbeat().SetIsStartup(false)
			want.GetHeartbeat().SetUptime(durationpb.New(time.Hour + time.Second))
			compare(t, want, got)
		default:
			t.Fatal("no heartbeat received")
		}

		cancel()
		assert.NoError(t, <-errCh)
	})
}

func ptr[T any](v T) *T { return &v }

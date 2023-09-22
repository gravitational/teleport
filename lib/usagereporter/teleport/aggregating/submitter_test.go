// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aggregating

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

// TestSubmitOnce tests the behavior of [submitOnce]; the [RunSubmitter]
// function is just a jittered periodic call to submitOnce, so testing it has
// very little use.
func TestSubmitOnce(t *testing.T) {
	ctx := context.Background()
	clk := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock:     clk,
		EventsOff: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	svc := reportService{bk}

	var submitted []*prehogv1.UserActivityReport
	submitOk := func(ctx context.Context, req *prehogv1.SubmitUsageReportsRequest) (uuid.UUID, error) {
		submitted = append(submitted, req.UserActivity...)
		return uuid.New(), nil
	}
	submitErr := func(ctx context.Context, req *prehogv1.SubmitUsageReportsRequest) (uuid.UUID, error) {
		submitted = append(submitted, req.UserActivity...)
		return uuid.Nil, errors.New("")
	}

	scfg := SubmitterConfig{
		Backend:   bk,
		Log:       logrus.StandardLogger(),
		Status:    local.NewStatusService(bk),
		Submitter: submitOk,
	}
	require.NoError(t, scfg.CheckAndSetDefaults())

	r0 := newReport(clk.Now())
	require.NoError(t, svc.upsertUserActivityReport(ctx, r0, reportTTL))

	// successful submit, no alerts, no leftover reports
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 1)
	require.True(t, proto.Equal(r0, submitted[0]))
	reports, err := svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, reports)
	submitted = nil

	alerts, err := scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Empty(t, alerts)

	require.NoError(t, svc.upsertUserActivityReport(ctx, r0, reportTTL))

	// failed submit past the grace time, we get the alert and the report is still there
	clk.Advance(alertGraceDuration)
	scfg.Submitter = submitErr
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 1)
	require.True(t, proto.Equal(r0, submitted[0]))
	reports, err = svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.True(t, proto.Equal(r0, reports[0]))
	submitted = nil

	alerts, err = scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, alertName, alerts[0].GetName())

	scfg.Submitter = submitOk
	// the lock is still held, nothing happens
	submitOnce(ctx, scfg)
	require.Empty(t, submitted)

	clk.Advance(submitLockDuration)
	// successful submission, no remaining events but the alert stays for one more cycle
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 1)
	submitted = nil

	alerts, err = scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, alertName, alerts[0].GetName())

	// nothing to submit, alert gone
	submitOnce(ctx, scfg)
	require.Empty(t, submitted)
	alerts, err = scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Empty(t, alerts)
}

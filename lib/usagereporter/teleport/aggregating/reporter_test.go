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
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func TestReporter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clk := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock: clk,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })
	// we set up a watcher to not have to poll the backend for newly added items
	// we expect
	w, err := bk.NewWatcher(ctx, backend.Watch{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, w.Close()) })
	recvBackendEvent := func() backend.Event {
		select {
		case e := <-w.Events():
			return e
		case <-time.After(time.Second):
			t.Fatal("failed to get backend event")
			return backend.Event{}
		}
	}
	require.Equal(t, types.OpInit, recvBackendEvent().Type)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "clustername",
	})
	require.NoError(t, err)

	r, err := NewReporter(ctx, ReporterConfig{
		Backend:     bk,
		Log:         logrus.StandardLogger(),
		Clock:       clk,
		ClusterName: clusterName,
		HostID:      uuid.NewString(),
	})
	require.NoError(t, err)

	svc := reportService{bk}

	r.ingested = make(chan usagereporter.Anonymizable, 3)
	recvIngested := func() {
		select {
		case <-r.ingested:
		case <-time.After(time.Second):
			t.Fatal("failed to receive ingested event")
		}
	}
	r.AnonymizeAndSubmit(&usagereporter.UserLoginEvent{
		UserName: "alice",
	})
	r.AnonymizeAndSubmit(&usagereporter.SessionStartEvent{
		UserName:    "alice",
		SessionType: "ssh",
	})
	r.AnonymizeAndSubmit(&usagereporter.SessionStartEvent{
		UserName:    "alice",
		SessionType: "ssh",
	})
	recvIngested()
	recvIngested()
	recvIngested()
	r.ingested = nil

	clk.BlockUntil(1)
	clk.Advance(reportGranularity)

	require.Equal(t, types.OpPut, recvBackendEvent().Type)

	reports, err := svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Len(t, reports[0].Records, 1)
	record := reports[0].Records[0]
	require.Equal(t, uint64(1), record.Logins)
	require.Equal(t, uint64(2), record.SshSessions)

	require.NoError(t, svc.deleteUserActivityReport(ctx, reports[0]))
	require.Equal(t, types.OpDelete, recvBackendEvent().Type)

	// on a GracefulStop there's no need to advance the clock, all processed
	// data is emitted immediately
	r.ingested = make(chan usagereporter.Anonymizable, 3)
	r.AnonymizeAndSubmit(&usagereporter.UserLoginEvent{
		UserName: "alice",
	})
	r.AnonymizeAndSubmit(&usagereporter.UserLoginEvent{
		UserName: "bob",
	})
	r.AnonymizeAndSubmit(&usagereporter.SessionStartEvent{
		UserName:    "bob",
		SessionType: "k8s",
	})
	recvIngested()
	recvIngested()
	recvIngested()
	r.ingested = nil

	require.NoError(t, r.GracefulStop(ctx))
	reports, err = svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Len(t, reports[0].Records, 2)
	rec1, rec2 := reports[0].Records[0], reports[0].Records[1]
	// record.UserName is alice
	if !bytes.Equal(record.UserName, rec1.UserName) {
		rec1, rec2 = rec2, rec1
	}
	require.Equal(t, uint64(1), rec1.Logins)
	require.Equal(t, uint64(1), rec2.Logins)
	require.Equal(t, uint64(0), rec1.KubeSessions)
	require.Equal(t, uint64(1), rec2.KubeSessions)
}

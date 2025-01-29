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

package aggregating

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
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

	anonymizer, err := utils.NewHMACAnonymizer("0123456789abcdef")
	require.NoError(t, err)

	r, err := NewReporter(ctx, ReporterConfig{
		Backend:     bk,
		Clock:       clk,
		ClusterName: clusterName,
		HostID:      uuid.NewString(),
		Anonymizer:  anonymizer,
	})
	require.NoError(t, err)

	svc := reportService{bk}

	r.ingested = make(chan usagereporter.Anonymizable, 4)
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
	r.AnonymizeAndSubmit(&usagereporter.SPIFFESVIDIssuedEvent{
		UserName: "alice",
	})
	recvIngested()
	recvIngested()
	recvIngested()
	recvIngested()

	clk.BlockUntil(1)
	clk.Advance(userActivityReportGranularity)

	require.Equal(t, types.OpPut, recvBackendEvent().Type)

	reports, err := svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Len(t, reports[0].Records, 1)
	record := reports[0].Records[0]
	require.Equal(t, uint64(1), record.Logins)
	require.Equal(t, uint64(2), record.SshSessions)
	require.Equal(t, uint64(1), record.SpiffeSvidsIssued)

	r.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   "srv01",
		Kind:   prehogv1a.ResourceKind_RESOURCE_KIND_NODE,
		Static: true,
	})
	recvIngested()

	clk.BlockUntil(1)
	clk.Advance(resourceReportGranularity)

	require.Equal(t, types.OpPut, recvBackendEvent().Type)

	resReports, err := svc.listResourcePresenceReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, resReports, 1)
	require.Len(t, resReports[0].ResourceKindReports, 1)
	resRecord := resReports[0].ResourceKindReports[0]
	require.Equal(t, prehogv1.ResourceKind_RESOURCE_KIND_NODE, resRecord.ResourceKind)
	require.Len(t, resRecord.ResourceIds, 1)

	require.NoError(t, svc.deleteUserActivityReport(ctx, reports[0]))
	require.Equal(t, types.OpDelete, recvBackendEvent().Type)
	require.NoError(t, svc.deleteResourcePresenceReport(ctx, resReports[0]))
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

func TestReporterBotInstanceActivity(t *testing.T) {
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

	anonymizer, err := utils.NewHMACAnonymizer("0123456789abcdef")
	require.NoError(t, err)

	r, err := NewReporter(ctx, ReporterConfig{
		Backend:     bk,
		Clock:       clk,
		ClusterName: clusterName,
		HostID:      uuid.NewString(),
		Anonymizer:  anonymizer,
	})
	require.NoError(t, err)

	svc := reportService{bk}

	r.ingested = make(chan usagereporter.Anonymizable, 4)
	recvIngested := func() {
		select {
		case <-r.ingested:
		case <-time.After(time.Second):
			t.Fatal("failed to receive ingested event")
		}
	}

	r.AnonymizeAndSubmit(&usagereporter.UserCertificateIssuedEvent{
		UserName:      "bot-bob",
		BotInstanceId: "0000-01",
		IsBot:         true,
	})
	r.AnonymizeAndSubmit(&usagereporter.BotJoinEvent{
		BotName:       "bob",
		BotInstanceId: "0000-01",
	})
	// Submit for two different bot instances, we expect a useractivity record
	// with a value of two, and two bot instance activity records with a single
	// value of 1.
	r.AnonymizeAndSubmit(&usagereporter.SPIFFESVIDIssuedEvent{
		UserName:      "bot-bob",
		BotInstanceId: "0000-01",
		UserKind:      prehogv1a.UserKind_USER_KIND_BOT,
	})
	r.AnonymizeAndSubmit(&usagereporter.SPIFFESVIDIssuedEvent{
		UserName:      "bot-bob",
		BotInstanceId: "0000-02",
		UserKind:      prehogv1a.UserKind_USER_KIND_BOT,
	})
	recvIngested()
	recvIngested()
	recvIngested()
	recvIngested()

	clk.BlockUntil(1)
	clk.Advance(botInstanceActivityReportGranularity)

	require.Equal(t, types.OpPut, recvBackendEvent().Type)
	require.Equal(t, types.OpPut, recvBackendEvent().Type)

	userActivityReports, err := svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, userActivityReports, 1)
	require.Len(t, userActivityReports[0].Records, 1)
	userActivityRecord := userActivityReports[0].Records[0]
	require.Equal(t, uint64(1), userActivityRecord.BotJoins)
	require.Equal(t, uint64(2), userActivityRecord.SpiffeSvidsIssued)
	require.Equal(t, uint64(1), userActivityRecord.CertificatesIssued)

	botInstanceActivityReports, err := svc.listBotInstanceActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, botInstanceActivityReports, 1)
	require.Len(t, botInstanceActivityReports[0].Records, 2)
	for _, record := range botInstanceActivityReports[0].Records {
		require.Equal(t, userActivityRecord.UserName, record.BotUserName)
	}
	require.True(t, slices.ContainsFunc(botInstanceActivityReports[0].Records, func(record *prehogv1.BotInstanceActivityRecord) bool {
		return record.BotJoins == 1 && record.CertificatesIssued == 1 && record.SpiffeSvidsIssued == 1
	}))
	require.True(t, slices.ContainsFunc(botInstanceActivityReports[0].Records, func(record *prehogv1.BotInstanceActivityRecord) bool {
		return record.BotJoins == 0 && record.CertificatesIssued == 0 && record.SpiffeSvidsIssued == 1
	}))
}

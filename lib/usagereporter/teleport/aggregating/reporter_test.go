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

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

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
		UserName:   "alice",
		UserOrigin: prehogv1a.UserOrigin_USER_ORIGIN_LOCAL,
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
	r.AnonymizeAndSubmit(&usagereporter.UserLoginEvent{
		UserName:   "alice",
		UserOrigin: prehogv1a.UserOrigin_USER_ORIGIN_UNSPECIFIED,
	})
	recvIngested()
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
	require.Equal(t, uint64(2), record.Logins)
	require.Equal(t, prehogv1.UserOrigin_USER_ORIGIN_LOCAL, record.GetUserOrigin())
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

func TestReporterMachineWorkloadIdentityActivity(t *testing.T) {
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
	r.AnonymizeAndSubmit(&usagereporter.SPIFFESVIDIssuedEvent{
		UserName: "jen",
		UserKind: prehogv1a.UserKind_USER_KIND_HUMAN,
		SpiffeId: "spiffe://clustername/jen",
	})
	// Submit for two different bot instances, we expect a useractivity record
	// with a value of two, and two bot instance activity records with a single
	// value of 1.
	r.AnonymizeAndSubmit(&usagereporter.SPIFFESVIDIssuedEvent{
		UserName:      "bot-bob",
		BotInstanceId: "0000-01",
		UserKind:      prehogv1a.UserKind_USER_KIND_BOT,
		SpiffeId:      "spiffe://clustername/bot/bob",
	})
	r.AnonymizeAndSubmit(&usagereporter.SPIFFESVIDIssuedEvent{
		UserName:      "bot-bob",
		BotInstanceId: "0000-01",
		UserKind:      prehogv1a.UserKind_USER_KIND_BOT,
		SpiffeId:      "spiffe://clustername/bot/bob-2",
	})
	r.AnonymizeAndSubmit(&usagereporter.SPIFFESVIDIssuedEvent{
		UserName:      "bot-bob",
		BotInstanceId: "0000-02",
		UserKind:      prehogv1a.UserKind_USER_KIND_BOT,
		SpiffeId:      "spiffe://clustername/bot/bob",
	})
	recvIngested()
	recvIngested()
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
	slices.SortFunc(userActivityReports[0].Records, func(a, b *prehogv1.UserActivityRecord) int {
		return bytes.Compare(a.GetUserName(), b.GetUserName())
	})
	want := []*prehogv1.UserActivityRecord{
		{
			UserName:           anonymizer.AnonymizeNonEmpty("bot-bob"),
			UserKind:           prehogv1.UserKind_USER_KIND_BOT,
			BotJoins:           1,
			SpiffeSvidsIssued:  3,
			CertificatesIssued: 1,
			SpiffeIdsIssued: []*prehogv1.SPIFFEIDRecord{
				{
					SpiffeId:    anonymizer.AnonymizeNonEmpty("spiffe://clustername/bot/bob"),
					SvidsIssued: 2,
				},
				{
					SpiffeId:    anonymizer.AnonymizeNonEmpty("spiffe://clustername/bot/bob-2"),
					SvidsIssued: 1,
				},
			},
		},
		{
			UserName:          anonymizer.AnonymizeNonEmpty("jen"),
			UserKind:          prehogv1.UserKind_USER_KIND_HUMAN,
			SpiffeSvidsIssued: 1,
			SpiffeIdsIssued: []*prehogv1.SPIFFEIDRecord{
				{
					SpiffeId:    anonymizer.AnonymizeNonEmpty("spiffe://clustername/jen"),
					SvidsIssued: 1,
				},
			},
		},
	}
	slices.SortFunc(want, func(a, b *prehogv1.UserActivityRecord) int {
		return bytes.Compare(a.GetUserName(), b.GetUserName())
	})
	diff := cmp.Diff(
		userActivityReports[0].Records,
		want,
		protocmp.Transform(),
		protocmp.SortRepeated(func(u1, u2 *prehogv1.SPIFFEIDRecord) bool {
			return bytes.Compare(u1.GetSpiffeId(), u2.GetSpiffeId()) == -1
		}),
	)
	if diff != "" {
		t.Errorf("UserActivityRecords mismatch (-want +got):\n%s", diff)
	}

	botUserRecordIndex := slices.IndexFunc(userActivityReports[0].Records, func(record *prehogv1.UserActivityRecord) bool {
		return bytes.Equal(record.UserName, anonymizer.AnonymizeNonEmpty("bot-bob"))
	})
	require.GreaterOrEqual(t, botUserRecordIndex, 0)
	botUserRecord := userActivityReports[0].Records[botUserRecordIndex]

	botInstanceActivityReports, err := svc.listBotInstanceActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, botInstanceActivityReports, 1)
	require.Len(t, botInstanceActivityReports[0].Records, 2)
	for _, record := range botInstanceActivityReports[0].Records {
		require.Equal(t, botUserRecord.UserName, record.BotUserName)
	}
	require.True(t, slices.ContainsFunc(botInstanceActivityReports[0].Records, func(record *prehogv1.BotInstanceActivityRecord) bool {
		return record.BotJoins == 1 && record.CertificatesIssued == 1 && record.SpiffeSvidsIssued == 2
	}))
	require.True(t, slices.ContainsFunc(botInstanceActivityReports[0].Records, func(record *prehogv1.BotInstanceActivityRecord) bool {
		return record.BotJoins == 0 && record.CertificatesIssued == 0 && record.SpiffeSvidsIssued == 1
	}))
}

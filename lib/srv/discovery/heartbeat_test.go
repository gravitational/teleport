// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
)

func TestBuildSelfHeartbeat(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	s := &Server{
		Config: &Config{
			ServerID:       "aaaaaaaa-1111-2222-3333-444444444444",
			Hostname:       "disc-1.example.com",
			DiscoveryGroup: "demo",
			PollInterval:   5 * time.Minute,
			Matchers: Matchers{
				AWS: []types.AWSMatcher{{
					Types:       []string{"ec2"},
					Regions:     []string{"us-east-1"},
					Integration: "my-oidc",
					Tags:        types.Labels{"env": []string{"prod"}},
				}},
				Azure: []types.AzureMatcher{{
					Types:         []string{"vm"},
					Regions:       []string{"eastus"},
					Subscriptions: []string{"sub-1"},
					Integration:   "azure-int",
				}},
			},
			clock: clock,
		},
	}

	first, err := s.buildSelfHeartbeat()
	require.NoError(t, err)

	require.Equal(t, "demo", first.GetSpec().GetDiscoveryGroup())
	require.Equal(t, s.ServerID, first.GetMetadata().GetName())
	require.Nil(t, first.GetMetadata().GetExpires(), "client must leave expiry for Auth to assign")
	require.Len(t, first.GetSpec().GetStaticMatchers().GetAws(), 1)
	require.False(t, first.GetSpec().GetMatchersTruncated())
}

// TestBuildSelfHeartbeatOmitsInstallerParams verifies that the heartbeat
// reports discovery selectors without post-discovery enrollment settings and
// does not mutate the matchers used by the service.
func TestBuildSelfHeartbeatOmitsInstallerParams(t *testing.T) {
	t.Parallel()

	s := &Server{
		Config: &Config{
			ServerID: "host-1",
			Matchers: Matchers{
				AWS: []types.AWSMatcher{{
					Types:   []string{"ec2"},
					Regions: []string{"us-east-1"},
					Params: &types.InstallerParams{
						JoinToken: "aws-token-name",
						HTTPProxySettings: &types.HTTPProxySettings{
							HTTPProxy:  "http://aws-user:aws-password@proxy.example.com:8080",
							HTTPSProxy: "https://aws-user:aws-password@proxy.example.com:8443/path?mode=connect",
							NoProxy:    "localhost,127.0.0.1",
						},
					}}},
				Azure: []types.AzureMatcher{{
					Types:   []string{"vm"},
					Regions: []string{"eastus"},
					Params: &types.InstallerParams{
						HTTPProxySettings: &types.HTTPProxySettings{
							HTTPSProxy: "https://azure-user@proxy.example.com:8443",
						},
					}}},
				GCP: []types.GCPMatcher{{
					Types:     []string{"vm"},
					Locations: []string{"us-central1"},
					Params: &types.InstallerParams{
						HTTPProxySettings: &types.HTTPProxySettings{
							HTTPProxy: "gcp-user:gcp-password@proxy.example.com:8080",
						},
					}}},
				Kubernetes: []types.KubernetesMatcher{{
					Types:      []string{"app"},
					Namespaces: []string{"production"},
				}},
			},
			clock: clockwork.NewFakeClock(),
		},
	}
	originalMatchers, err := json.Marshal(s.Matchers)
	require.NoError(t, err)

	hb, err := s.buildSelfHeartbeat()
	require.NoError(t, err)
	matchers := hb.GetSpec().GetStaticMatchers()
	require.Nil(t, matchers.GetAws()[0].Params)
	require.Nil(t, matchers.GetAzure()[0].Params)
	require.Nil(t, matchers.GetGcp()[0].Params)
	require.Equal(t, []string{"ec2"}, matchers.GetAws()[0].Types)
	require.Equal(t, []string{"us-east-1"}, matchers.GetAws()[0].Regions)
	require.Equal(t, []string{"vm"}, matchers.GetAzure()[0].Types)
	require.Equal(t, []string{"eastus"}, matchers.GetAzure()[0].Regions)
	require.Equal(t, []string{"vm"}, matchers.GetGcp()[0].Types)
	require.Equal(t, []string{"us-central1"}, matchers.GetGcp()[0].Locations)
	require.Equal(t, []string{"app"}, matchers.GetKube()[0].Types)
	require.Equal(t, []string{"production"}, matchers.GetKube()[0].Namespaces)

	encoded, err := json.Marshal(hb)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "aws-token-name")
	require.NotContains(t, string(encoded), "aws-password")
	require.NotContains(t, string(encoded), "azure-user")
	require.NotContains(t, string(encoded), "gcp-password")

	afterBuild, err := json.Marshal(s.Matchers)
	require.NoError(t, err)
	require.Equal(t, originalMatchers, afterBuild, "building a heartbeat must not mutate runtime matchers")

	// The projected matcher must not point directly at the live slice element.
	s.Matchers.Kubernetes[0] = types.KubernetesMatcher{Types: []string{"service"}}
	require.Equal(t, []string{"app"}, matchers.GetKube()[0].Types)
}

// TestDiscardUnsupportedMatchers verifies that heartbeats report the effective
// static matcher set after unsupported matchers have been filtered.
func TestDiscardUnsupportedMatchers(t *testing.T) {
	t.Parallel()

	makeMatchers := func() Matchers {
		return Matchers{
			AWS: []types.AWSMatcher{
				{Types: []string{"ec2"}, Integration: "int-1"},
				{Types: []string{"rds"}}, // no integration -> discarded
				{Types: []string{"eks"}, Integration: "int-2"},
			},
			Azure: []types.AzureMatcher{
				{Types: []string{"vm"}}, // no integration -> discarded
				{Types: []string{"aks"}, Integration: "int-3"},
			},
			GCP: []types.GCPMatcher{
				{Types: []string{"gce"}}, // all GCP discarded
			},
		}
	}

	t.Run("integration-only mode reports effective matchers", func(t *testing.T) {
		s := &Server{Config: &Config{
			IntegrationOnlyCredentials: true,
			Log:                        slog.Default(),
			clock:                      clockwork.NewFakeClock(),
		}}
		s.ctx = t.Context()

		m := makeMatchers()
		s.discardUnsupportedMatchers(&m)

		require.Len(t, m.AWS, 2, "effective AWS set keeps only integration matchers")
		require.Len(t, m.Azure, 1)
		require.Empty(t, m.GCP)

		s.Matchers = m
		hb, err := s.buildSelfHeartbeat()
		require.NoError(t, err)
		require.Len(t, hb.GetSpec().GetStaticMatchers().GetAws(), 2)
		require.Len(t, hb.GetSpec().GetStaticMatchers().GetAzure(), 1)
		require.Empty(t, hb.GetSpec().GetStaticMatchers().GetGcp())
	})

	t.Run("self-hosted mode discards nothing", func(t *testing.T) {
		s := &Server{Config: &Config{
			IntegrationOnlyCredentials: false,
			Log:                        slog.Default(),
		}}
		s.ctx = t.Context()

		m := makeMatchers()
		s.discardUnsupportedMatchers(&m)
		require.Len(t, m.AWS, 3, "matchers untouched")
	})
}

// TestStaticMatcherTruncation verifies the size-budget fallback: an
// over-budget matcher set is replaced by per-cloud counts with the truncation
// flag set; never a silently shortened list.
func TestStaticMatcherTruncation(t *testing.T) {
	t.Parallel()

	huge := make([]types.AWSMatcher, 0, 512)
	for range 512 {
		huge = append(huge, types.AWSMatcher{
			Types:       []string{"ec2", "rds", "eks"},
			Regions:     []string{"us-east-1", "us-west-2", "eu-central-1"},
			Integration: strings.Repeat("integration-name-", 8),
			Tags:        types.Labels{"env": []string{strings.Repeat("v", 64)}},
		})
	}
	s := &Server{
		Config: &Config{
			ServerID: "host-1",
			Matchers: Matchers{
				AWS: huge,
				GCP: []types.GCPMatcher{{Types: []string{"gce"}}},
				AccessGraph: &types.AccessGraphSync{
					AWS:   []*types.AccessGraphAWSSync{{}, {}},
					Azure: []*types.AccessGraphAzureSync{{}, {}, {}},
				},
			},
			clock: clockwork.NewFakeClock(),
		},
	}

	hb, err := s.buildSelfHeartbeat()
	require.NoError(t, err)
	spec := hb.GetSpec()
	require.True(t, spec.GetMatchersTruncated(), "over-budget matchers must set the truncation flag")
	require.Nil(t, spec.GetStaticMatchers(), "matcher detail must be dropped wholesale, not shortened")
	require.EqualValues(t, 512, spec.GetStaticMatcherCounts()[services.StaticMatcherCountKeyAWS])
	require.EqualValues(t, 1, spec.GetStaticMatcherCounts()[services.StaticMatcherCountKeyGCP])
	require.EqualValues(t, 5, spec.GetStaticMatcherCounts()[services.StaticMatcherCountKeyAccessGraph])
}

func TestMatcherTruncationStarted(t *testing.T) {
	var state heartbeatAnnounceState
	require.True(t, state.matcherTruncationStarted(true), "initial truncation must be reported")
	require.False(t, state.matcherTruncationStarted(true), "continued truncation must stay quiet")
	require.False(t, state.matcherTruncationStarted(false), "recovery must not be reported as truncation")
	require.True(t, state.matcherTruncationStarted(true), "truncation after recovery must be reported again")
}

type fakeAnnouncer struct {
	calls    chan struct{}
	services chan *discoveryservicev1.DiscoveryService
	err      atomic.Pointer[error]
}

func newFakeAnnouncer(err error) *fakeAnnouncer {
	fa := &fakeAnnouncer{
		calls:    make(chan struct{}, 100),
		services: make(chan *discoveryservicev1.DiscoveryService, 100),
	}
	fa.err.Store(&err)
	return fa
}

func (f *fakeAnnouncer) UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error) {
	f.services <- svc
	select {
	case f.calls <- struct{}{}:
	case <-ctx.Done():
	}
	return svc, *f.err.Load()
}

func newAnnouncerTestServer(t *testing.T, announcer Announcer, clock clockwork.Clock) *Server {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return &Server{
		Config: &Config{
			ServerID:                  "host-1",
			Log:                       slog.Default(),
			clock:                     clock,
			jitter:                    retryutils.SeventhJitter,
			heartbeatJitter:           func(time.Duration) time.Duration { return time.Second },
			DiscoveryServiceAnnouncer: announcer,
		},
		ctx:      ctx,
		cancelfn: cancel,
	}
}

// expectAnnounce waits for the bubble to go idle and requires that an
// announce attempt has happened. Must run inside a synctest bubble.
func expectAnnounce(t *testing.T, fa *fakeAnnouncer, msg string) {
	t.Helper()
	synctest.Wait()
	select {
	case <-fa.calls:
	default:
		t.Fatal(msg)
	}
}

// expectNoAnnounce waits for the bubble to go idle and requires that no
// announce attempt has happened. Deterministic under synctest: idleness
// means no goroutine can still be on its way to announcing, so absence is
// proof rather than a scheduling accident. Must run inside a synctest
// bubble.
func expectNoAnnounce(t *testing.T, fa *fakeAnnouncer, msg string) {
	t.Helper()
	synctest.Wait()
	select {
	case <-fa.calls:
		t.Fatal(msg)
	default:
	}
}

// TestAnnouncerRenews validates the steady-state loop: announce at start,
// silence between renewals, and a renewal after the announce period elapses.
func TestAnnouncerRenews(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fa := newFakeAnnouncer(nil)
		s := newAnnouncerTestServer(t, fa, clockwork.NewRealClock())

		s.startHeartbeatAnnouncer()
		expectAnnounce(t, fa, "expected an initial announce")

		time.Sleep(heartbeatCheckPeriod)
		expectNoAnnounce(t, fa, "must not announce before renewal is due")

		time.Sleep(heartbeatTTL/2 + time.Second)
		expectAnnounce(t, fa, "expected a renewal announce after the announce period")
	})
}

// TestAnnouncerNotImplemented validates the compatibility behavior: on
// NotImplemented from an older auth server the announcer goes quiet and
// uses the ordinary retry schedule so heartbeating resumes after an auth
// upgrade without an agent restart.
func TestAnnouncerNotImplemented(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fa := newFakeAnnouncer(trace.NotImplemented("old auth"))
		s := newAnnouncerTestServer(t, fa, clockwork.NewRealClock())

		s.startHeartbeatAnnouncer()
		expectAnnounce(t, fa, "expected an initial announce attempt")

		time.Sleep(heartbeatRetryPeriod)
		expectNoAnnounce(t, fa, "retry jitter must be observed by the ticker")

		fa.err.Store(new(error))
		time.Sleep(heartbeatCheckPeriod)
		expectAnnounce(t, fa, "expected an ordinary retry after NotImplemented")
	})
}

// TestAnnouncerRetriesOnTransientError validates that ordinary announce
// failures retry on the retry period rather than every check period.
func TestAnnouncerRetriesOnTransientError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fa := newFakeAnnouncer(trace.ConnectionProblem(nil, "boom"))
		s := newAnnouncerTestServer(t, fa, clockwork.NewRealClock())

		s.startHeartbeatAnnouncer()
		expectAnnounce(t, fa, "expected an initial announce attempt")

		time.Sleep(heartbeatCheckPeriod)
		expectNoAnnounce(t, fa, "failed announce must back off, not retry every check period")

		time.Sleep(heartbeatRetryPeriod - heartbeatCheckPeriod)
		expectNoAnnounce(t, fa, "retry must include positive jitter")

		time.Sleep(heartbeatCheckPeriod)
		expectAnnounce(t, fa, "expected retry when the ticker observes its deadline")
	})
}

// TestAnnouncerRebuildsSnapshotForRetry verifies that a due attempt rebuilds
// from current producer state. v1 inputs are startup-fixed in production, but
// this prevents a future runtime-varying field from being frozen at startup.
// gatedAnnouncer holds each RPC open until the test releases it, so a test
// can mutate producer state while the announcer is provably blocked inside
// the RPC: the release send is a happens-before edge ordering the mutation
// before the announcer's next snapshot read. synctest.Wait alone does not
// provide that edge; it only reports quiescence.
type gatedAnnouncer struct {
	fa      *fakeAnnouncer
	release chan struct{}
}

func (g *gatedAnnouncer) UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error) {
	res, err := g.fa.UpsertDiscoveryService(ctx, svc)
	select {
	case <-g.release:
	case <-ctx.Done():
	}
	return res, err
}

func TestAnnouncerRebuildsSnapshotForRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fa := newFakeAnnouncer(trace.ConnectionProblem(nil, "boom"))
		ga := &gatedAnnouncer{fa: fa, release: make(chan struct{})}
		s := newAnnouncerTestServer(t, ga, clockwork.NewRealClock())
		s.DiscoveryGroup = "before"
		s.startHeartbeatAnnouncer()
		expectAnnounce(t, fa, "expected initial attempt")
		require.Equal(t, "before", (<-fa.services).GetSpec().GetDiscoveryGroup())

		// The announcer is blocked inside the gated RPC; this write is
		// ordered before its next snapshot read by the release send below.
		s.DiscoveryGroup = "changed-after-start"
		ga.release <- struct{}{}

		time.Sleep(heartbeatCheckPeriod)
		expectNoAnnounce(t, fa, "retry backoff must suppress attempts")
		fa.err.Store(new(error))
		time.Sleep(heartbeatRetryPeriod)
		expectAnnounce(t, fa, "expected retry")
		require.Equal(t, "changed-after-start", (<-fa.services).GetSpec().GetDiscoveryGroup(), "retry must rebuild from current producer state")
		ga.release <- struct{}{}
	})
}

type deadlineAnnouncer struct {
	started chan time.Time
}

func (d *deadlineAnnouncer) UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, trace.BadParameter("missing deadline")
	}
	d.started <- deadline
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestAnnouncerRPCDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		da := &deadlineAnnouncer{started: make(chan time.Time, 1)}
		s := newAnnouncerTestServer(t, da, clockwork.NewRealClock())
		start := time.Now()
		s.startHeartbeatAnnouncer()

		synctest.Wait()
		select {
		case deadline := <-da.started:
			require.Equal(t, start.Add(heartbeatRPCTimeout), deadline)
		default:
			t.Fatal("RPC did not start")
		}

		time.Sleep(heartbeatCheckPeriod)
		synctest.Wait()
		select {
		case <-da.started:
			t.Fatal("a second RPC started while the first was still in flight")
		default:
		}
	})
}

// cancelingAnnouncer cancels the server context from inside the RPC,
// simulating a shutdown racing an in-flight announce.
type cancelingAnnouncer struct{ cancel context.CancelFunc }

func (c cancelingAnnouncer) UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error) {
	c.cancel()
	return nil, context.Canceled
}

// TestAnnouncerShutdownQuiet validates that a shutdown canceling an
// in-flight announce RPC is not logged as an announce failure: the loop is
// exiting, and a spurious "Failed to announce" on every shutdown trains
// operators to ignore the warning that matters.
func TestAnnouncerShutdownQuiet(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var quiet bytes.Buffer
		s := newAnnouncerTestServer(t, nil, clockwork.NewRealClock())
		s.Log = slog.New(slog.NewTextHandler(&quiet, nil))
		s.DiscoveryServiceAnnouncer = cancelingAnnouncer{cancel: s.cancelfn}

		var state heartbeatAnnounceState
		s.announceHeartbeatOnce(time.Now(), &state)
		require.NotContains(t, quiet.String(), "Failed to announce",
			"shutdown-canceled announce must not be logged as a failure")

		// Control: the same failure with a live server context must log,
		// proving the capture above can observe the warning at all.
		var noisy bytes.Buffer
		s2 := newAnnouncerTestServer(t, newFakeAnnouncer(trace.BadParameter("boom")), clockwork.NewRealClock())
		s2.Log = slog.New(slog.NewTextHandler(&noisy, nil))

		var state2 heartbeatAnnounceState
		s2.announceHeartbeatOnce(time.Now(), &state2)
		require.Contains(t, noisy.String(), "Failed to announce")
	})
}

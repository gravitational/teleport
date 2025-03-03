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
	"net"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func ptr[T any](v T) *T {
	return &v
}

func TestSPIFFEWorkloadAPIService_filterSVIDRequests(t *testing.T) {
	// This test is more for overall behavior. Use the _field test for
	// each individual field.
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	tests := []struct {
		name string
		att  *workloadidentityv1pb.WorkloadAttrs
		in   []config.SVIDRequestWithRules
		want []config.SVIDRequest
	}{
		{
			name: "no rules",
			in: []config.SVIDRequestWithRules{
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/foo",
					},
				},
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/bar",
					},
				},
			},
			want: []config.SVIDRequest{
				{
					Path: "/foo",
				},
				{
					Path: "/bar",
				},
			},
		},
		{
			name: "no rules with attestation",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []config.SVIDRequestWithRules{
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/foo",
					},
				},
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/bar",
					},
				},
			},
			want: []config.SVIDRequest{
				{
					Path: "/foo",
				},
				{
					Path: "/bar",
				},
			},
		},
		{
			name: "no rules with attestation",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					// We don't expect that workloadattest will ever return
					// Attested: false and include UID/PID/GID but we want to
					// ensure we handle this by failing regardless.
					Attested: false,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []config.SVIDRequestWithRules{
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/foo",
					},
					Rules: []config.SVIDRequestRule{
						{
							Unix: config.SVIDRequestRuleUnix{
								UID: ptr(1000),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "no matching rules with attestation",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []config.SVIDRequestWithRules{
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/foo",
					},
					Rules: []config.SVIDRequestRule{
						{
							Unix: config.SVIDRequestRuleUnix{
								UID: ptr(1000),
								PID: ptr(1),
							},
						},
						{
							Unix: config.SVIDRequestRuleUnix{
								GID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/bar",
					},
					Rules: []config.SVIDRequestRule{
						{
							Unix: config.SVIDRequestRuleUnix{
								UID: ptr(1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "no matching rules without attestation",
			in: []config.SVIDRequestWithRules{
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/foo",
					},
					Rules: []config.SVIDRequestRule{
						{
							Unix: config.SVIDRequestRuleUnix{
								PID: ptr(1),
							},
						},
						{
							Unix: config.SVIDRequestRuleUnix{
								GID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/bar",
					},
					Rules: []config.SVIDRequestRule{
						{
							Unix: config.SVIDRequestRuleUnix{
								UID: ptr(1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "some matching rules with uds",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []config.SVIDRequestWithRules{
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/fizz",
					},
					Rules: []config.SVIDRequestRule{
						{
							Unix: config.SVIDRequestRuleUnix{
								UID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/foo",
					},
					Rules: []config.SVIDRequestRule{},
				},
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/bar",
					},
					Rules: []config.SVIDRequestRule{
						{
							Unix: config.SVIDRequestRuleUnix{
								UID: ptr(1000),
								GID: ptr(1500),
							},
						},
						{
							Unix: config.SVIDRequestRuleUnix{
								UID: ptr(1000),
								PID: ptr(1002),
							},
						},
					},
				},
			},
			want: []config.SVIDRequest{
				{
					Path: "/foo",
				},
				{
					Path: "/bar",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSVIDRequests(ctx, log, tt.in, tt.att)
			assert.Empty(t, gocmp.Diff(tt.want, got))
		})
	}
}

func TestSPIFFEWorkloadAPIService_filterSVIDRequests_field(t *testing.T) {
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	tests := []struct {
		field       string
		matching    *workloadidentityv1pb.WorkloadAttrs
		nonMatching *workloadidentityv1pb.WorkloadAttrs
		rule        config.SVIDRequestRule
	}{
		{
			field: "unix.pid",
			rule: config.SVIDRequestRule{
				Unix: config.SVIDRequestRuleUnix{
					PID: ptr(1000),
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Pid:      1000,
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Pid:      200,
				},
			},
		},
		{
			field: "unix.uid",
			rule: config.SVIDRequestRule{
				Unix: config.SVIDRequestRuleUnix{
					UID: ptr(1000),
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      200,
				},
			},
		},
		{
			field: "unix.gid",
			rule: config.SVIDRequestRule{
				Unix: config.SVIDRequestRuleUnix{
					GID: ptr(1000),
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Gid:      1000,
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Gid:      200,
				},
			},
		},
		{
			field: "unix.namespace",
			rule: config.SVIDRequestRule{
				Kubernetes: config.SVIDRequestRuleKubernetes{
					Namespace: "foo",
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:  true,
					Namespace: "foo",
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:  true,
					Namespace: "bar",
				},
			},
		},
		{
			field: "kubernetes.service_account",
			rule: config.SVIDRequestRule{
				Kubernetes: config.SVIDRequestRuleKubernetes{
					ServiceAccount: "foo",
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:       true,
					ServiceAccount: "foo",
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:       true,
					ServiceAccount: "bar",
				},
			},
		},
		{
			field: "kubernetes.pod_name",
			rule: config.SVIDRequestRule{
				Kubernetes: config.SVIDRequestRuleKubernetes{
					PodName: "foo",
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested: true,
					PodName:  "foo",
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested: true,
					PodName:  "bar",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			rules := []config.SVIDRequestWithRules{
				{
					SVIDRequest: config.SVIDRequest{
						Path: "/foo",
					},
					Rules: []config.SVIDRequestRule{tt.rule},
				},
			}
			t.Run("matching", func(t *testing.T) {
				assert.Len(t, filterSVIDRequests(ctx, log, rules, tt.matching), 1)
			})
			t.Run("non-matching", func(t *testing.T) {
				assert.Empty(t, filterSVIDRequests(ctx, log, rules, tt.nonMatching))
			})
		})
	}
}

// TestBotSPIFFEWorkloadAPI is an end-to-end test of Workload ID's ability to
// issue a SPIFFE SVID to a workload connecting via the SPIFFE Workload API.
func TestBotSPIFFEWorkloadAPI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	// Make a new auth server.
	process := testenv.MakeTestServer(t, defaultTestServerOpts(t, log))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Create a role that allows the bot to issue a SPIFFE SVID.
	role, err := types.NewRole("spiffe-issuer", types.RoleSpecV6{
		Allow: types.RoleConditions{
			SPIFFE: []*types.SPIFFERoleCondition{
				{
					Path: "/*",
					DNSSANs: []string{
						"*",
					},
					IPSANs: []string{
						"0.0.0.0/0",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	pid := os.Getpid()

	tempDir := t.TempDir()
	socketPath := "unix://" + path.Join(tempDir, "spiffe.sock")
	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())
	botConfig := defaultBotConfig(
		t, process, onboarding, config.ServiceConfigs{
			&config.SPIFFEWorkloadAPIService{
				Listen: socketPath,
				SVIDs: []config.SVIDRequestWithRules{
					// Intentionally unmatching PID to ensure this SVID
					// is not issued.
					{
						SVIDRequest: config.SVIDRequest{
							Path: "/bar",
						},
						Rules: []config.SVIDRequestRule{
							{
								Unix: config.SVIDRequestRuleUnix{
									PID: ptr(0),
								},
							},
						},
					},
					// SVID with rule that matches on PID.
					{
						SVIDRequest: config.SVIDRequest{
							Path: "/foo",
							Hint: "hint",
							SANS: config.SVIDRequestSANs{
								DNS: []string{"example.com"},
								IP:  []string{"10.0.0.1"},
							},
						},
						Rules: []config.SVIDRequestRule{
							{
								Unix: config.SVIDRequestRuleUnix{
									PID: &pid,
								},
							},
						},
					},
				},
			},
		},
		defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
		},
	)
	botConfig.Oneshot = false
	b := New(botConfig, log)

	// Spin up goroutine for bot to run in
	botCtx, cancelBot := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(botCtx)
		assert.NoError(t, err, "bot should not exit with error")
		cancelBot()
	}()
	t.Cleanup(func() {
		// Shut down bot and make sure it exits.
		cancelBot()
		wg.Wait()
	})

	t.Run("X509", func(t *testing.T) {
		t.Parallel()

		// This has a little flexibility internally in terms of waiting for the
		// socket to come up, so we don't need a manual sleep/retry here.
		source, err := workloadapi.NewX509Source(
			ctx,
			workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)),
		)
		require.NoError(t, err)
		defer source.Close()

		svid, err := source.GetX509SVID()
		require.NoError(t, err)

		// SVID has successfully been issued. We can now assert that it's correct.
		require.Equal(t, "spiffe://root/foo", svid.ID.String())
		cert := svid.Certificates[0]
		require.Equal(t, "spiffe://root/foo", cert.URIs[0].String())
		require.True(t, net.IPv4(10, 0, 0, 1).Equal(cert.IPAddresses[0]))
		require.Equal(t, []string{"example.com"}, cert.DNSNames)
		require.WithinRange(
			t,
			cert.NotAfter,
			cert.NotBefore.Add(time.Hour-time.Minute),
			cert.NotBefore.Add(time.Hour+time.Minute),
		)
	})

	t.Run("JWT", func(t *testing.T) {
		t.Parallel()

		source, err := workloadapi.NewJWTSource(
			ctx,
			workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)),
		)
		require.NoError(t, err)
		defer source.Close()

		validateSVID := func(
			t *testing.T,
			svid *jwtsvid.SVID,
			wantAudience string,
		) {
			t.Helper()
			// First, check the response fields
			require.Equal(t, "spiffe://root/foo", svid.ID.String())
			require.Equal(t, "hint", svid.Hint)

			// Validate "locally" that the SVID is correct.
			validatedSVID, err := jwtsvid.ParseAndValidate(
				svid.Marshal(),
				source,
				[]string{wantAudience},
			)
			require.NoError(t, err)
			require.Equal(t, svid.Claims, validatedSVID.Claims)
			require.Equal(t, svid.ID, validatedSVID.ID)

			// Validate "remotely" that the SVID is correct using the Workload
			// API.
			validatedSVID, err = workloadapi.ValidateJWTSVID(
				ctx,
				svid.Marshal(),
				wantAudience,
				workloadapi.WithAddr(socketPath),
			)
			require.NoError(t, err)
			require.Equal(t, svid.Claims, validatedSVID.Claims)
			require.Equal(t, svid.ID, validatedSVID.ID)
		}

		svids, err := source.FetchJWTSVIDs(ctx, jwtsvid.Params{
			Audience:       "example.com",
			ExtraAudiences: []string{"2.example.com"},
			Subject:        spiffeid.RequireFromString("spiffe://root/foo"),
		})
		require.NoError(t, err)
		require.Len(t, svids, 1)
		validateSVID(t, svids[0], "2.example.com")

		// Try again with no specified subject (e.g receive all)
		svids, err = source.FetchJWTSVIDs(ctx, jwtsvid.Params{
			Audience: "example.com",
		})
		require.NoError(t, err)
		require.Len(t, svids, 1)
		validateSVID(t, svids[0], "example.com")
	})
}

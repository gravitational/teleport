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
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/spiffe/workloadattest"
	"github.com/gravitational/teleport/lib/utils"
)

func ptr[T any](v T) *T {
	return &v
}

func TestSPIFFEWorkloadAPIService_filterSVIDRequests(t *testing.T) {
	// This test is more for overall behaviour. Use the _field test for
	// each individual field.
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	tests := []struct {
		name string
		att  workloadattest.Attestation
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
			att: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					UID:      1000,
					GID:      1001,
					PID:      1002,
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
			att: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					// We don't expect that workloadattest will ever return
					// Attested: false and include UID/PID/GID but we want to
					// ensure we handle this by failing regardless.
					Attested: false,
					UID:      1000,
					GID:      1001,
					PID:      1002,
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
			att: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					UID:      1000,
					GID:      1001,
					PID:      1002,
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
			att: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					UID:      1000,
					GID:      1001,
					PID:      1002,
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
		matching    workloadattest.Attestation
		nonMatching workloadattest.Attestation
		rule        config.SVIDRequestRule
	}{
		{
			field: "unix.pid",
			rule: config.SVIDRequestRule{
				Unix: config.SVIDRequestRuleUnix{
					PID: ptr(1000),
				},
			},
			matching: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					PID:      1000,
				},
			},
			nonMatching: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					PID:      200,
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
			matching: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					UID:      1000,
				},
			},
			nonMatching: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					UID:      200,
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
			matching: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					GID:      1000,
				},
			},
			nonMatching: workloadattest.Attestation{
				Unix: workloadattest.UnixAttestation{
					Attested: true,
					GID:      200,
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
			matching: workloadattest.Attestation{
				Kubernetes: workloadattest.KubernetesAttestation{
					Attested:  true,
					Namespace: "foo",
				},
			},
			nonMatching: workloadattest.Attestation{
				Kubernetes: workloadattest.KubernetesAttestation{
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
			matching: workloadattest.Attestation{
				Kubernetes: workloadattest.KubernetesAttestation{
					Attested:       true,
					ServiceAccount: "foo",
				},
			},
			nonMatching: workloadattest.Attestation{
				Kubernetes: workloadattest.KubernetesAttestation{
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
			matching: workloadattest.Attestation{
				Kubernetes: workloadattest.KubernetesAttestation{
					Attested: true,
					PodName:  "foo",
				},
			},
			nonMatching: workloadattest.Attestation{
				Kubernetes: workloadattest.KubernetesAttestation{
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

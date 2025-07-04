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

package legacyspiffe

import (
	"context"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/attrs"
	"github.com/gravitational/teleport/lib/utils"
)

func ptr[T any](v T) *T {
	return &v
}

func TestFilterSVIDRequests(t *testing.T) {
	// This test is more for overall behavior. Use the _field test for
	// each individual field.
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	tests := []struct {
		name string
		att  *workloadidentityv1pb.WorkloadAttrs
		in   []SVIDRequestWithRules
		want []SVIDRequest
	}{
		{
			name: "no rules",
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
				},
			},
			want: []SVIDRequest{
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
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
				},
			},
			want: []SVIDRequest{
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
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
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
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1000),
								PID: ptr(1),
							},
						},
						{
							Unix: SVIDRequestRuleUnix{
								GID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
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
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								PID: ptr(1),
							},
						},
						{
							Unix: SVIDRequestRuleUnix{
								GID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
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
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/fizz",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1000),
								GID: ptr(1500),
							},
						},
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1000),
								PID: ptr(1002),
							},
						},
					},
				},
			},
			want: []SVIDRequest{
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
			got := filterSVIDRequests(ctx, log, tt.in, attrs.FromWorkloadAttrs(tt.att))
			assert.Empty(t, gocmp.Diff(tt.want, got))
		})
	}
}

func TestFilterSVIDRequests_field(t *testing.T) {
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	tests := []struct {
		field       string
		matching    *workloadidentityv1pb.WorkloadAttrs
		nonMatching *workloadidentityv1pb.WorkloadAttrs
		rule        SVIDRequestRule
	}{
		{
			field: "unix.pid",
			rule: SVIDRequestRule{
				Unix: SVIDRequestRuleUnix{
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
			rule: SVIDRequestRule{
				Unix: SVIDRequestRuleUnix{
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
			rule: SVIDRequestRule{
				Unix: SVIDRequestRuleUnix{
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
			rule: SVIDRequestRule{
				Kubernetes: SVIDRequestRuleKubernetes{
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
			rule: SVIDRequestRule{
				Kubernetes: SVIDRequestRuleKubernetes{
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
			rule: SVIDRequestRule{
				Kubernetes: SVIDRequestRuleKubernetes{
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
			rules := []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{tt.rule},
				},
			}
			t.Run("matching", func(t *testing.T) {
				assert.Len(t, filterSVIDRequests(ctx, log, rules, attrs.FromWorkloadAttrs(tt.matching)), 1)
			})
			t.Run("non-matching", func(t *testing.T) {
				assert.Empty(t, filterSVIDRequests(ctx, log, rules, attrs.FromWorkloadAttrs(tt.nonMatching)))
			})
		})
	}
}

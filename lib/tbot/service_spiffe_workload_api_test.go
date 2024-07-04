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
	"github.com/gravitational/teleport/lib/uds"
	"github.com/gravitational/teleport/lib/utils"
)

func ptr[T any](v T) *T {
	return &v
}

func TestSPIFFEWorkloadAPIService_filterSVIDRequests(t *testing.T) {
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	tests := []struct {
		name string
		uds  *uds.Creds
		in   []config.SVIDRequestWithRules
		want []config.SVIDRequest
	}{
		{
			name: "no rules",
			uds:  nil,
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
			name: "no rules with uds",
			uds: &uds.Creds{
				UID: 1000,
				GID: 1001,
				PID: 1002,
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
			name: "no matching rules with uds",
			uds: &uds.Creds{
				UID: 1000,
				GID: 1001,
				PID: 1002,
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
			name: "no matching rules without uds",
			uds:  nil,
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
			uds: &uds.Creds{
				UID: 1000,
				GID: 1001,
				PID: 1002,
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
			got := filterSVIDRequests(ctx, log, tt.in, tt.uds)
			assert.Empty(t, gocmp.Diff(tt.want, got))
		})
	}
}

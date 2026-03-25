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

package autoupdate

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestNewAutoUpdateAgentReport(t *testing.T) {
	now := timestamppb.Now()
	expires := timestamppb.New(now.AsTime().Add(autoUpdateReportTTL))
	tests := []struct {
		name     string
		spec     *autoupdate.AutoUpdateAgentReportSpec
		authName string

		want    *autoupdate.AutoUpdateAgentReport
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:     "nil spec",
			authName: "test",
			wantErr:  require.Error,
		},
		{
			name: "empty name",
			spec: &autoupdate.AutoUpdateAgentReportSpec{
				Timestamp: now,
			},
			wantErr: require.Error,
		},
		{
			name:     "no timestamp",
			authName: "test",
			spec:     &autoupdate.AutoUpdateAgentReportSpec{},
			wantErr:  require.Error,
		},
		{
			name:     "ok",
			authName: "test",
			spec: &autoupdate.AutoUpdateAgentReportSpec{
				Timestamp: now,
			},
			want: &autoupdate.AutoUpdateAgentReport{
				Kind:    types.KindAutoUpdateAgentReport,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "test",
					Expires: expires,
				},
				Spec: &autoupdate.AutoUpdateAgentReportSpec{
					Timestamp: now,
				},
			},
			wantErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewAutoUpdateAgentReport(tt.spec, tt.authName)
			tt.wantErr(t, err)
			require.Empty(t, cmp.Diff(tt.want, result, protocmp.Transform()))
		})
	}
}

func TestNewAutoUpdateBotInstanceReport(t *testing.T) {
	now := timestamppb.Now()
	expires := timestamppb.New(now.AsTime().Add(autoUpdateReportTTL))

	generateGroups := func(t *testing.T, numGroups, numVersions int) map[string]*autoupdate.AutoUpdateBotInstanceReportSpecGroup {
		t.Helper()

		groups := make(map[string]*autoupdate.AutoUpdateBotInstanceReportSpecGroup, numGroups)
		for groupIdx := range numGroups {
			group := &autoupdate.AutoUpdateBotInstanceReportSpecGroup{
				Versions: make(map[string]*autoupdate.AutoUpdateBotInstanceReportSpecGroupVersion, numVersions),
			}
			for versionIdx := range numVersions {
				group.Versions[strconv.Itoa(versionIdx)] = &autoupdate.AutoUpdateBotInstanceReportSpecGroupVersion{Count: 1}
			}
			groups[strconv.Itoa(groupIdx)] = group
		}
		return groups
	}

	tests := []struct {
		name string
		spec *autoupdate.AutoUpdateBotInstanceReportSpec

		want    *autoupdate.AutoUpdateBotInstanceReport
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "nil spec",
			wantErr: require.Error,
		},
		{
			name:    "no timestamp",
			spec:    &autoupdate.AutoUpdateBotInstanceReportSpec{},
			wantErr: require.Error,
		},
		{
			name: "too many groups",
			spec: &autoupdate.AutoUpdateBotInstanceReportSpec{
				Timestamp: now,
				Groups:    generateGroups(t, 51, 0),
			},
			wantErr: require.Error,
		},
		{
			name: "too many versions",
			spec: &autoupdate.AutoUpdateBotInstanceReportSpec{
				Timestamp: now,
				Groups:    generateGroups(t, 1, 21),
			},
			wantErr: require.Error,
		},
		{
			name: "ok",
			spec: &autoupdate.AutoUpdateBotInstanceReportSpec{
				Timestamp: now,
				Groups:    generateGroups(t, 1, 1),
			},
			want: &autoupdate.AutoUpdateBotInstanceReport{
				Kind:    types.KindAutoUpdateBotInstanceReport,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    types.MetaNameAutoUpdateBotInstanceReport,
					Expires: expires,
				},
				Spec: &autoupdate.AutoUpdateBotInstanceReportSpec{
					Timestamp: now,
					Groups:    generateGroups(t, 1, 1),
				},
			},
			wantErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewAutoUpdateBotInstanceReport(tt.spec)
			tt.wantErr(t, err)
			require.Empty(t, cmp.Diff(tt.want, result, protocmp.Transform()))
		})
	}
}

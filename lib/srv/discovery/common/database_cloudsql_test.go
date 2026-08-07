/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestSetGCPDBName(t *testing.T) {
	tests := []struct {
		desc           string
		labels         map[string]string
		firstNamePart  string
		extraNameParts []string
		wantName       string
	}{
		{
			desc:          "no override label, single part",
			firstNamePart: "my-instance",
			wantName:      "my-instance",
		},
		{
			desc:           "no override label, extra parts",
			firstNamePart:  "my-instance",
			extraNameParts: []string{"x"},
			wantName:       "my-instance-x",
		},
		{
			desc: "override label replaces first part",
			labels: map[string]string{
				types.GCPDatabaseNameOverrideLabel: "custom",
			},
			firstNamePart: "my-instance",
			wantName:      "custom",
		},
		{
			desc: "override label replaces first part, extra parts appended",
			labels: map[string]string{
				types.GCPDatabaseNameOverrideLabel: "custom",
			},
			firstNamePart:  "my-instance",
			extraNameParts: []string{"x"},
			wantName:       "custom-x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			meta := types.Metadata{
				Name:   "original",
				Labels: tt.labels,
			}
			got := setGCPDBName(meta, tt.firstNamePart, tt.extraNameParts...)
			require.Equal(t, tt.wantName, got.Name)
		})
	}
}

func TestDiscoverCloudSQLDatabases(t *testing.T) {
	runnable := func(name string) *sqladmin.DatabaseInstance {
		return &sqladmin.DatabaseInstance{
			Name:            name,
			Project:         "proj-1",
			Region:          "us-central1",
			State:           "RUNNABLE",
			DatabaseVersion: "POSTGRES_14",
			InstanceType:    "CLOUD_SQL_INSTANCE",
			IpAddresses:     []*sqladmin.IpMapping{{Type: "PRIMARY", IpAddress: "1.2.3.4"}},
		}
	}
	unavailable := runnable("unavailable")
	unavailable.State = "SUSPENDED"
	unsupportedType := runnable("unsupported-type")
	unsupportedType.InstanceType = "ON_PREMISES_INSTANCE"
	noEndpoint := runnable("no-endpoint")
	noEndpoint.IpAddresses = nil

	instances := []*sqladmin.DatabaseInstance{
		runnable("a"),
		unavailable,
		unsupportedType,
		noEndpoint,
		runnable("b"),
	}

	databases := DiscoverCloudSQLDatabases(t.Context(), logtest.NewLogger(), instances)
	require.Len(t, databases, 2)
	require.Equal(t, "a", databases[0].GetGCP().InstanceID)
	require.Equal(t, "b", databases[1].GetGCP().InstanceID)
}

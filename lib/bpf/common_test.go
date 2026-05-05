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

package bpf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// TestCheckAndSetDefaults makes sure defaults are set when the user does not
// provide values for the page sizes and hard coded values (like zero or a
// specific page size) are respected when given.
func TestBPFConfig_CheckAndSetDefaults(t *testing.T) {
	perfBufferPageCount := defaults.PerfBufferPageCount
	openPerfBufferPageCount := defaults.OpenPerfBufferPageCount
	zeroPageCount := 0

	var tests = []struct {
		name string
		got  *servicecfg.BPFConfig
		want *servicecfg.BPFConfig
	}{
		{
			name: "all defaults",
			got:  &servicecfg.BPFConfig{},
			want: &servicecfg.BPFConfig{
				CommandBufferSize: &perfBufferPageCount,
				DiskBufferSize:    &openPerfBufferPageCount,
				NetworkBufferSize: &perfBufferPageCount,
				CgroupPath:        defaults.CgroupPath,
			},
		},
		{
			name: "values set",
			got: &servicecfg.BPFConfig{
				CommandBufferSize: &zeroPageCount,
				DiskBufferSize:    &zeroPageCount,
				NetworkBufferSize: &perfBufferPageCount,
				CgroupPath:        "/my/cgroup/",
			},
			want: &servicecfg.BPFConfig{
				CommandBufferSize: &zeroPageCount,
				DiskBufferSize:    &zeroPageCount,
				NetworkBufferSize: &perfBufferPageCount,
				CgroupPath:        "/my/cgroup/",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.got.CheckAndSetDefaults()
			require.NoError(t, err, "CheckAndSetDefaults errored")

			if diff := cmp.Diff(test.want, test.got); diff != "" {
				t.Errorf("CheckAndSetDefaults mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

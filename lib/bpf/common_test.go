/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bpf

import (
	"testing"
	"time"

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
	udpSilencePeriod := defaults.UDPSilencePeriod
	udpSilenceBufferSize := defaults.UDPSilenceBufferSize
	customUDPSilencePeriod := 1 * time.Minute
	customUDPSilenceBufferSize := 42

	var tests = []struct {
		name string
		got  *servicecfg.BPFConfig
		want *servicecfg.BPFConfig
	}{
		{
			name: "all defaults",
			got:  &servicecfg.BPFConfig{},
			want: &servicecfg.BPFConfig{
				CommandBufferSize:    &perfBufferPageCount,
				DiskBufferSize:       &openPerfBufferPageCount,
				NetworkBufferSize:    &perfBufferPageCount,
				CgroupPath:           defaults.CgroupPath,
				UDPSilencePeriod:     &udpSilencePeriod,
				UDPSilenceBufferSize: &udpSilenceBufferSize,
			},
		},
		{
			name: "values set",
			got: &servicecfg.BPFConfig{
				CommandBufferSize:    &zeroPageCount,
				DiskBufferSize:       &zeroPageCount,
				NetworkBufferSize:    &perfBufferPageCount,
				CgroupPath:           "/my/cgroup/",
				UDPSilencePeriod:     &customUDPSilencePeriod,
				UDPSilenceBufferSize: &customUDPSilenceBufferSize,
			},
			want: &servicecfg.BPFConfig{
				CommandBufferSize:    &zeroPageCount,
				DiskBufferSize:       &zeroPageCount,
				NetworkBufferSize:    &perfBufferPageCount,
				CgroupPath:           "/my/cgroup/",
				UDPSilencePeriod:     &customUDPSilencePeriod,
				UDPSilenceBufferSize: &customUDPSilenceBufferSize,
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

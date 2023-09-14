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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// TestCheckAndSetDefaults makes sure defaults are set when the user does not
// provide values for the page sizes and hard coded values (like zero or a
// specific page size) are respected when given.
func TestCheckAndSetDefaults(t *testing.T) {
	var perfBufferPageCount = defaults.PerfBufferPageCount
	var openPerfBufferPageCount = defaults.OpenPerfBufferPageCount
	var zeroPageCount = 0

	var tests = []struct {
		inConfig  *servicecfg.BPFConfig
		outConfig *servicecfg.BPFConfig
	}{
		// Empty values get defaults.
		{
			inConfig: &servicecfg.BPFConfig{
				CommandBufferSize: nil,
				DiskBufferSize:    nil,
				NetworkBufferSize: nil,
			},
			outConfig: &servicecfg.BPFConfig{
				CommandBufferSize: &perfBufferPageCount,
				DiskBufferSize:    &openPerfBufferPageCount,
				NetworkBufferSize: &perfBufferPageCount,
			},
		},
		// Values are not wiped out with defaults.
		{
			inConfig: &servicecfg.BPFConfig{
				CommandBufferSize: &zeroPageCount,
				DiskBufferSize:    &zeroPageCount,
				NetworkBufferSize: &perfBufferPageCount,
			},
			outConfig: &servicecfg.BPFConfig{
				CommandBufferSize: &zeroPageCount,
				DiskBufferSize:    &zeroPageCount,
				NetworkBufferSize: &perfBufferPageCount,
			},
		},
	}

	for _, tt := range tests {
		err := tt.inConfig.CheckAndSetDefaults()
		require.NoError(t, err)
		require.Equal(t, *tt.outConfig.CommandBufferSize, *tt.inConfig.CommandBufferSize)
		require.Equal(t, *tt.outConfig.DiskBufferSize, *tt.inConfig.DiskBufferSize)
		require.Equal(t, *tt.outConfig.NetworkBufferSize, *tt.inConfig.NetworkBufferSize)
	}
}

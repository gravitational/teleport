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
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type CommonSuite struct{}

var _ = check.Suite(&CommonSuite{})

// TestCheckAndSetDefaults makes sure defaults are set when the user does not
// provide values for the page sizes and hard coded values (like zero or a
// specific page size) are respected when given.
func (s *CommonSuite) TestCheckAndSetDefaults(c *check.C) {
	var perfBufferPageCount = defaults.PerfBufferPageCount
	var openPerfBufferPageCount = defaults.OpenPerfBufferPageCount
	var zeroPageCount = 0

	var tests = []struct {
		inConfig  *Config
		outConfig *Config
	}{
		// Empty values get defaults.
		{
			inConfig: &Config{
				CommandBufferSize: nil,
				DiskBufferSize:    nil,
				NetworkBufferSize: nil,
			},
			outConfig: &Config{
				CommandBufferSize: &perfBufferPageCount,
				DiskBufferSize:    &openPerfBufferPageCount,
				NetworkBufferSize: &perfBufferPageCount,
			},
		},
		// Values are not wiped out with defaults.
		{
			inConfig: &Config{
				CommandBufferSize: &zeroPageCount,
				DiskBufferSize:    &zeroPageCount,
				NetworkBufferSize: &perfBufferPageCount,
			},
			outConfig: &Config{
				CommandBufferSize: &zeroPageCount,
				DiskBufferSize:    &zeroPageCount,
				NetworkBufferSize: &perfBufferPageCount,
			},
		},
	}

	for _, tt := range tests {
		err := tt.inConfig.CheckAndSetDefaults()
		c.Assert(err, check.IsNil)
		c.Assert(*tt.inConfig.CommandBufferSize, check.Equals, *tt.outConfig.CommandBufferSize)
		c.Assert(*tt.inConfig.DiskBufferSize, check.Equals, *tt.outConfig.DiskBufferSize)
		c.Assert(*tt.inConfig.NetworkBufferSize, check.Equals, *tt.outConfig.NetworkBufferSize)
	}
}

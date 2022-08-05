/*
Copyright 2020 Gravitational, Inc.

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

package utils

import (
	"strings"

	"github.com/coreos/go-semver/semver"
	"gopkg.in/check.v1"
)

type KernelSuite struct{}

var _ = check.Suite(&KernelSuite{})

// TestKernelVersion checks that version strings for various distributions
// can be parsed correctly.
func (s *KernelSuite) TestKernelVersion(c *check.C) {
	var tests = []struct {
		inRelease  string
		inMin      string
		inMax      string
		outRelease string
	}{
		// Debian 10
		{
			inRelease:  "4.19.0-6-cloud-amd64",
			inMin:      "4.18.0",
			inMax:      "4.20.0",
			outRelease: "4.19.0",
		},
		// CentOS 6
		{
			inRelease:  "4.19.94",
			inMin:      "4.18.0",
			inMax:      "4.20.0",
			outRelease: "4.19.94",
		},
		// CentOS 7
		{
			inRelease:  "4.19.72-25.58.amzn2.x86_64",
			inMin:      "4.18.0",
			inMax:      "4.20.0",
			outRelease: "4.19.72",
		},
		// CentOS 8
		{
			inRelease:  "4.18.0-80.11.2.el8_0.x86_64",
			inMin:      "4.17.0",
			inMax:      "4.29.0",
			outRelease: "4.18.0",
		},
		// Ubuntu 19.04
		{
			inRelease:  "5.0.0-1028-gcp",
			inMin:      "4.18.0",
			inMax:      "5.1.0",
			outRelease: "5.0.0",
		},
	}

	for _, tt := range tests {
		// Check the version is parsed correctly.
		version, err := kernelVersion(strings.NewReader(tt.inRelease))
		c.Assert(err, check.IsNil)
		c.Assert(version.String(), check.Equals, tt.outRelease)

		// Check that version comparisons work.
		min, err := semver.NewVersion(tt.inMin)
		c.Assert(err, check.IsNil)
		max, err := semver.NewVersion(tt.inMax)
		c.Assert(err, check.IsNil)
		c.Assert(version.LessThan(*max), check.Equals, true)
		c.Assert(version.LessThan(*min), check.Equals, false)
	}
}

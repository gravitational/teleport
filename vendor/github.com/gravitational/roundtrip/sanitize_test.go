/*
Copyright 2018 Gravitational, Inc.

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

package roundtrip

import (
	"fmt"
	"net/url"
	"testing"

	"gopkg.in/check.v1"
)

func TestSanitizer(t *testing.T) { check.TestingT(t) }

type SanitizeSuite struct {
}

var _ = check.Suite(&SanitizeSuite{})
var _ = fmt.Printf

func (s *SanitizeSuite) SetUpSuite(c *check.C) {
}

func (s *SanitizeSuite) TearDownSuite(c *check.C) {
}

func (s *SanitizeSuite) TearDownTest(c *check.C) {
}

func (s *SanitizeSuite) SetUpTest(c *check.C) {
}

func (s *SanitizeSuite) TestSanitizePath(c *check.C) {
	tests := []struct {
		inPath   string
		outError bool
	}{
		{
			inPath:   "http://example.com:3080/hello",
			outError: false,
		},
		{
			inPath:   "http://example.com:3080/hello/../world",
			outError: true,
		},
		{
			inPath:   fmt.Sprintf("http://localhost:3080/hello/%v/goodbye", url.PathEscape("..")),
			outError: true,
		},
		{
			inPath:   "http://example.com:3080/hello?foo=..",
			outError: false,
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		err := isPathSafe(tt.inPath)
		if tt.outError {
			c.Assert(err, check.NotNil, comment)
		} else {
			c.Assert(err, check.IsNil, comment)
		}
	}
}

/*
Copyright 2017 Gravitational, Inc.

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
package proxy

import (
	"fmt"
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type ProxySuite struct{}

var _ = check.Suite(&ProxySuite{})
var _ = fmt.Printf

func (s *ProxySuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *ProxySuite) TearDownSuite(c *check.C) {}
func (s *ProxySuite) SetUpTest(c *check.C)     {}
func (s *ProxySuite) TearDownTest(c *check.C)  {}

func (s *ProxySuite) TestGetProxyAddress(c *check.C) {
	var tests = []struct {
		inEnvName    string
		inEnvValue   string
		outProxyAddr string
	}{
		// 0 - valid, can be raw host:port
		{
			"http_proxy",
			"proxy:1234",
			"proxy:1234",
		},
		// 1 - valid, raw host:port works for https
		{
			"HTTPS_PROXY",
			"proxy:1234",
			"proxy:1234",
		},
		// 2 - valid, correct full url
		{
			"https_proxy",
			"https://proxy:1234",
			"proxy:1234",
		},
		// 3 - valid, http endpoint can be set in https_proxy
		{
			"https_proxy",
			"http://proxy:1234",
			"proxy:1234",
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		unsetEnv()
		os.Setenv(tt.inEnvName, tt.inEnvValue)
		p := getProxyAddress()
		unsetEnv()

		c.Assert(p, check.Equals, tt.outProxyAddr, comment)
	}
}

func unsetEnv() {
	for _, envname := range []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"} {
		os.Unsetenv(envname)
	}
}

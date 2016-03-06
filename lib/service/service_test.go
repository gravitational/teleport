/*
Copyright 2015 Gravitational, Inc.

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
package service

import (
	"os"

	"gopkg.in/check.v1"
)

type ServiceTestSuite struct {
}

var _ = check.Suite(&ServiceTestSuite{})

func (s *ServiceTestSuite) SetUpSuite(c *check.C) {
}

func (s *ServiceTestSuite) TestSelfSignedHTTPS(c *check.C) {
	fileExists := func(fp string) bool {
		_, err := os.Stat(fp)
		if err != nil && os.IsNotExist(err) {
			return false
		}
		return true
	}
	cfg := &Config{
		DataDir:  c.MkDir(),
		Hostname: "example.com",
	}
	err := initSelfSignedHTTPSCert(cfg)
	c.Assert(err, check.IsNil)
	c.Assert(fileExists(cfg.Proxy.TLSCert), check.Equals, true)
	c.Assert(fileExists(cfg.Proxy.TLSKey), check.Equals, true)
}

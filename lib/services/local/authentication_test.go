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

package local

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type ClusterAuthPreferenceSuite struct {
	bk      backend.Backend
	tempDir string
}

var _ = check.Suite(&ClusterAuthPreferenceSuite{})
var _ = fmt.Printf

func (s *ClusterAuthPreferenceSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *ClusterAuthPreferenceSuite) TearDownSuite(c *check.C) {
}

func (s *ClusterAuthPreferenceSuite) SetUpTest(c *check.C) {
	var err error

	s.tempDir, err = ioutil.TempDir("", "preference-test-")
	c.Assert(err, check.IsNil)

	s.bk, err = boltbk.New(backend.Params{"path": s.tempDir})
	c.Assert(err, check.IsNil)
}

func (s *ClusterAuthPreferenceSuite) TearDownTest(c *check.C) {
	var err error

	c.Assert(s.bk.Close(), check.IsNil)

	err = os.RemoveAll(s.tempDir)
	c.Assert(err, check.IsNil)
}

func (s *ClusterAuthPreferenceSuite) TestCycle(c *check.C) {
	caps := NewClusterAuthPreferenceService(s.bk)

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "otp",
	})
	c.Assert(err, check.IsNil)

	err = caps.SetClusterAuthPreference(ap)
	c.Assert(err, check.IsNil)

	gotAP, err := caps.GetClusterAuthPreference()
	c.Assert(err, check.IsNil)

	c.Assert(gotAP.GetType(), check.Equals, "local")
	c.Assert(gotAP.GetSecondFactor(), check.Equals, "otp")
}

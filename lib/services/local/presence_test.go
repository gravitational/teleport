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
	"github.com/gravitational/teleport/lib/backend/dir"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"gopkg.in/check.v1"
)

type PresenceSuite struct {
	bk      backend.Backend
	tempDir string
}

var _ = check.Suite(&PresenceSuite{})
var _ = fmt.Printf

func (s *PresenceSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *PresenceSuite) TearDownSuite(c *check.C) {
}

func (s *PresenceSuite) SetUpTest(c *check.C) {
	var err error

	s.tempDir, err = ioutil.TempDir("", "trusted-clusters-")
	c.Assert(err, check.IsNil)

	s.bk, err = dir.New(backend.Params{"path": s.tempDir})
	c.Assert(err, check.IsNil)
}

func (s *PresenceSuite) TearDownTest(c *check.C) {
	var err error

	c.Assert(s.bk.Close(), check.IsNil)

	err = os.RemoveAll(s.tempDir)
	c.Assert(err, check.IsNil)
}

func (s *PresenceSuite) TestTrustedClusterCRUD(c *check.C) {
	presenceBackend := NewPresenceService(s.bk)

	tc, err := services.NewTrustedCluster("foo", services.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	c.Assert(err, check.IsNil)

	// we just insert this one for get all
	stc, err := services.NewTrustedCluster("bar", services.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	c.Assert(err, check.IsNil)

	// create trusted clusters
	_, err = presenceBackend.UpsertTrustedCluster(tc)
	c.Assert(err, check.IsNil)
	_, err = presenceBackend.UpsertTrustedCluster(stc)
	c.Assert(err, check.IsNil)

	// get trusted cluster make sure it's correct
	gotTC, err := presenceBackend.GetTrustedCluster("foo")
	c.Assert(err, check.IsNil)
	c.Assert(gotTC.GetName(), check.Equals, "foo")
	c.Assert(gotTC.GetEnabled(), check.Equals, true)
	c.Assert(gotTC.GetRoles(), check.DeepEquals, []string{"bar", "baz"})
	c.Assert(gotTC.GetToken(), check.Equals, "qux")
	c.Assert(gotTC.GetProxyAddress(), check.Equals, "quux")
	c.Assert(gotTC.GetReverseTunnelAddress(), check.Equals, "quuz")

	// get all clusters
	allTC, err := presenceBackend.GetTrustedClusters()
	c.Assert(err, check.IsNil)
	c.Assert(allTC, check.HasLen, 2)

	// delete cluster
	err = presenceBackend.DeleteTrustedCluster("foo")
	c.Assert(err, check.IsNil)

	// make sure it's really gone
	gotTC, err = presenceBackend.GetTrustedCluster("foo")
	c.Assert(err, check.NotNil)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}

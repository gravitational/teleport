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

package services

import (
	"fmt"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type TrustedClusterSuite struct{}

var _ = check.Suite(&TrustedClusterSuite{})
var _ = fmt.Printf

func (s *TrustedClusterSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *TrustedClusterSuite) TearDownSuite(c *check.C) {
}

func (s *TrustedClusterSuite) SetUpTest(c *check.C) {
}

func (s *TrustedClusterSuite) TearDownTest(c *check.C) {
}

func (s *TrustedClusterSuite) TestUnmarshal(c *check.C) {
	input := `
      {
        "kind": "trusted_cluster",
        "metadata": {
          "name": "foo"
        },
        "spec": {
          "enabled": true,
          "roles": ["bar", "baz"],
          "token": "qux",
          "web_proxy_addr": "quux",
          "tunnel_addr": "quuz"
        }
      }
	`

	output := TrustedClusterV2{
		Kind:    KindTrustedCluster,
		Version: V2,
		Metadata: Metadata{
			Name:      "foo",
			Namespace: defaults.Namespace,
		},
		Spec: TrustedClusterSpecV2{
			Enabled:              true,
			Roles:                []string{"bar", "baz"},
			Token:                "qux",
			ProxyAddress:         "quux",
			ReverseTunnelAddress: "quuz",
		},
	}

	ap, err := GetTrustedClusterMarshaler().Unmarshal([]byte(input))
	c.Assert(err, check.IsNil)
	c.Assert(ap, check.DeepEquals, &output)
}

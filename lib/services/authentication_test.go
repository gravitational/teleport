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

type ClusterAuthPreferenceSuite struct{}

var _ = check.Suite(&ClusterAuthPreferenceSuite{})
var _ = fmt.Printf

func (s *ClusterAuthPreferenceSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *ClusterAuthPreferenceSuite) TestUnmarshal(c *check.C) {
	input := `
      {
        "kind": "cluster_auth_preference",
        "metadata": {
          "name": "cluster-auth-preference"
        },
        "spec": {
          "type": "local",
          "second_factor": "otp"
        }
      }
	`

	output := AuthPreferenceV2{
		Kind:    KindClusterAuthPreference,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameClusterAuthPreference,
			Namespace: defaults.Namespace,
		},
		Spec: AuthPreferenceSpecV2{
			Type:         "local",
			SecondFactor: "otp",
		},
	}

	ap, err := GetAuthPreferenceMarshaler().Unmarshal([]byte(input))
	c.Assert(err, check.IsNil)
	c.Assert(ap, check.DeepEquals, &output)
}

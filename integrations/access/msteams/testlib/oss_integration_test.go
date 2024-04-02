// Copyright 2024 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testlib

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

func TestMsTeamsPluginOSS(t *testing.T) {
	teamsSuite := &MsTeamsSuite{
		AccessRequestSuite: &integration.AccessRequestSuite{
			AuthHelper: &integration.OSSAuthHelper{},
		},
	}
	suite.Run(t, teamsSuite)
}

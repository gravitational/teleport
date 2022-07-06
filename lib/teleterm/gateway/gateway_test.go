// Copyright 2022 Gravitational, Inc
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

package gateway

import (
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/stretchr/testify/require"
)

type mockCLICommandProvider struct{}

func (m mockCLICommandProvider) GetCommand(gateway *Gateway) (string, error) {
	command := fmt.Sprintf("%s/%s", gateway.TargetName, gateway.TargetSubresourceName)
	return command, nil
}

func TestCLICommandUsesCLICommandProvider(t *testing.T) {
	gateway := Gateway{
		Config: Config{
			TargetName:            "foo",
			TargetSubresourceName: "bar",
			Protocol:              defaults.ProtocolPostgres,
		},
		cliCommandProvider: mockCLICommandProvider{},
	}

	command, err := gateway.CLICommand()
	require.NoError(t, err)

	require.Equal(t, "foo/bar", command)
}

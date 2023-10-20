/*
Copyright 2023 Gravitational, Inc.

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

package awsoidc

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeployServiceConfig(t *testing.T) {
	t.Run("ensure log level is set to debug", func(t *testing.T) {
		base64Config, err := generateTeleportConfigString(DeployServiceRequest{
			ProxyServerHostPort:  "host:port",
			TeleportIAMTokenName: stringPointer("iam-token"),
			DeploymentMode:       DatabaseServiceDeploymentMode,
		})
		require.NoError(t, err)

		// Config must have the following string:
		// severity: debug

		base64SeverityDebug := base64.StdEncoding.EncodeToString([]byte("severity: debug"))
		require.Contains(t, base64Config, base64SeverityDebug)
	})
}

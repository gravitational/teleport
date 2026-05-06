/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKubeMessageTemplate(t *testing.T) {
	tts := []struct {
		name  string
		input map[string]any
		want  string
	}{
		{
			name: "token without secret",
			input: map[string]any{
				"proxy_server": "proxy.example.com:443",
				"token":        "token",
				"minutes":      5,
				"set_roles":    "kube",
				"version":      "1.2.3",
			},
			want: `The invite token: token
This token will expire in 5 minutes.

To use with Helm installation follow these steps:

# Retrieve the Teleport helm charts
helm repo add teleport https://charts.releases.teleport.dev
# Refresh the helm charts
helm repo update

> helm install teleport-agent teleport/teleport-kube-agent \
  --set kubeClusterName=cluster ` + "`# Change kubeClusterName variable to your preferred name.`" + ` \
  --set roles="kube" \
  --set proxyAddr=proxy.example.com:443 \
  --set authToken=token \
  --set updater.enabled=true \
  --create-namespace \
  --namespace=teleport-agent \
  --version=1.2.3

Please note:

  - This invitation token will expire in 5 minutes.
  - proxy.example.com:443 must be reachable from Kubernetes cluster.
  - The token is usable in a standalone Linux server with kubernetes_service.
  - See https://goteleport.com/docs/kubernetes-access/ for detailed installation information.

`,
		},
		{
			name: "token with secret",
			input: map[string]any{
				"proxy_server": "proxy.example.com:443",
				"token":        "token",
				"secret":       "secret",
				"minutes":      5,
				"set_roles":    "kube",
				"version":      "1.2.3",
			},
			want: `The invite token: token
This token will expire in 5 minutes.

To use with Helm installation follow these steps:

# Retrieve the Teleport helm charts
helm repo add teleport https://charts.releases.teleport.dev
# Refresh the helm charts
helm repo update

> helm install teleport-agent teleport/teleport-kube-agent \
  --set kubeClusterName=cluster ` + "`# Change kubeClusterName variable to your preferred name.`" + ` \
  --set roles="kube" \
  --set proxyAddr=proxy.example.com:443 \
  --set "joinParams.tokenName=token" \
  --set "joinParams.tokenSecret=secret" \
  --set "extraEnv[0].name=TELEPORT_UNSTABLE_SCOPES" \
  --set "extraEnv[0].value=yes" \
  --set updater.enabled=true \
  --create-namespace \
  --namespace=teleport-agent \
  --version=1.2.3

Please note:

  - This invitation token will expire in 5 minutes.
  - proxy.example.com:443 must be reachable from Kubernetes cluster.
  - The token is usable in a standalone Linux server with kubernetes_service.
  - See https://goteleport.com/docs/kubernetes-access/ for detailed installation information.

`,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			err := kubeMessageTemplate.Execute(buf, tt.input)
			require.NoError(t, err)

			require.Equal(t, tt.want, buf.String())
		})
	}
}

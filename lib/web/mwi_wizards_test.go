/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestMachineIDWizard(t *testing.T) {
	t.Parallel()

	const testJWKS = `{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "P9Zd",
      "alg": "RS256",
      "n": "kWp2zRA23Z3vTL4uoe8kTFptxBVFunIoP4t_8TDYJrOb7D1iZNDXVeEsYKp6ppmrTZDAgd-cNOTKLd4M39WJc5FN0maTAVKJc7NxklDeKc4dMe1BGvTZNG4MpWBo-taKULlYUu0ltYJuLzOjIrTHfarucrGoRWqM0sl3z2-fv9k",
      "e": "AQAB"
    }
  ]
}`

	env := newWebPack(t, 1)
	pack := env.proxies[0].authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "machine-id", "wizards", "ci-cd")

	testCases := map[string]machineIDWizardGenerateIaCRequest{
		"github+kubernetes-empty-terraform": {
			SourceType:      "github",
			DestinationType: "kubernetes",
		},
		"github+kubernetes-step1-terraform": {
			SourceType:      "github",
			DestinationType: "kubernetes",
			GitHub: &machineIDWizardRequestGitHub{
				Allow: []machineIDWizardRequestGitHubAllow{
					{
						Repository: "teleport",
						Owner:      "gravitational",
					},
				},
			},
		},
		"github+kubernetes-step2-terraform": {
			SourceType:      "github",
			DestinationType: "kubernetes",
			GitHub: &machineIDWizardRequestGitHub{
				Allow: []machineIDWizardRequestGitHubAllow{
					{
						Repository: "teleport",
						Owner:      "gravitational",
					},
				},
			},
			Kubernetes: &machineIDWizardRequestKubernetes{
				Labels: types.Labels{"department": []string{"engineering"}},
				Groups: []string{"system:masters"},
			},
		},
		"github+kubernetes-complex-terraform": {
			SourceType:      "github",
			DestinationType: "kubernetes",
			GitHub: &machineIDWizardRequestGitHub{
				Allow: []machineIDWizardRequestGitHubAllow{
					{
						Repository:  "teleport",
						Owner:       "gravitational",
						Workflow:    "deploy",
						Environment: "production",
						Actor:       "deployinator",
						Ref:         "main",
						RefType:     "branch",
					},
				},
				EnterpriseServerHost: "github.corp.internal",
				EnterpriseSlug:       "sluggy",
				StaticJWKS:           testJWKS,
			},
			Kubernetes: &machineIDWizardRequestKubernetes{
				Labels: types.Labels{"department": []string{"engineering"}},
				Groups: []string{"viewers"},
				Users:  []string{"serviceAccount:deployment-sa"},
				Resources: []types.KubernetesResource{
					{
						Kind:      "deployment",
						APIGroup:  "apps",
						Name:      "billing-service",
						Namespace: "default",
					},
				},
			},
		},
	}
	for name, req := range testCases {
		t.Run(name, func(t *testing.T) {
			rsp, err := pack.clt.PostJSON(t.Context(), endpoint, req)
			require.NoError(t, err)

			var body machineIDGHAK8sWizardGenerateIaCResponse
			require.NoError(t, json.Unmarshal(rsp.Bytes(), &body))

			// Cut the provider configuration out of the golden file, as this
			// contains values that will change between test runs (e.g. major
			// version number and proxy address).
			providerConfigIdx := strings.Index(body.Terraform, "terraform {")
			beforeProviderConfig := body.Terraform[:providerConfigIdx]
			providerConfig := body.Terraform[providerConfigIdx:]

			if golden.ShouldSet() {
				golden.Set(t, []byte(beforeProviderConfig))
			}

			require.Empty(t,
				cmp.Diff(
					string(golden.Get(t)),
					beforeProviderConfig,
				),
			)

			require.Contains(t,
				providerConfig,
				env.proxies[0].handler.handler.PublicProxyAddr(),
			)
		})
	}
}

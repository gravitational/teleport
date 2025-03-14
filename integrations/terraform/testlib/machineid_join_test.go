/*
Copyright 2024 Gravitational, Inc.

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

package testlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/testing/fakejoin"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	kubetoken "github.com/gravitational/teleport/lib/kube/token"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/integrations/terraform/provider"
)

func TestTerraformJoin(t *testing.T) {
	require.NoError(t, os.Setenv("TF_ACC", "true"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Test setup: start a Telpeort auth server
	authHelper := &integration.MinimalAuthHelper{}
	clt := authHelper.StartServer(t)

	var err error
	// Test setup: create the terraform role
	tfRole := services.NewPresetTerraformProviderRole()
	tfRole, err = clt.CreateRole(ctx, tfRole)
	require.NoError(t, err)

	// Test setup: create a fake Kubernetes signer that will allow us to use the kubernetes/jwks join method
	clock := clockwork.NewRealClock()
	signer, err := fakejoin.NewKubernetesSigner(clock)
	require.NoError(t, err)

	jwks, err := signer.GetMarshaledJWKS()
	require.NoError(t, err)

	// Test setup: create a token and a bot that can join the cluster with JWT signed by our fake Kubernetes signer
	testBotName := "testBot"
	testTokenName := "testToken"
	fakeNamespace := "test-namespace"
	fakeServiceAccount := "test-service-account"
	token, err := types.NewProvisionTokenFromSpec(
		testTokenName,
		clock.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles:      types.SystemRoles{types.RoleBot},
			JoinMethod: types.JoinMethodKubernetes,
			BotName:    testBotName,
			Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
				Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
					{
						ServiceAccount: fmt.Sprintf("%s:%s", fakeNamespace, fakeServiceAccount),
					},
				},
				Type: types.KubernetesJoinTypeStaticJWKS,
				StaticJWKS: &types.ProvisionTokenSpecV2Kubernetes_StaticJWKSConfig{
					JWKS: jwks,
				},
			},
		})
	require.NoError(t, err)
	err = clt.CreateToken(ctx, token)
	require.NoError(t, err)

	bot := &machineidv1.Bot{
		Metadata: &headerv1.Metadata{
			Name: testBotName,
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{tfRole.GetName()},
		},
	}
	_, err = clt.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{Bot: bot})
	require.NoError(t, err)

	// Test setup: sign a Kube JWT for our bot to join the cluster
	// We sign the token, write it to a temporary file, and point the embedded tbot to it
	// with an environment variable.
	pong, err := clt.Ping(ctx)
	require.NoError(t, err)
	clusterName := pong.ClusterName
	jwt, err := signer.SignServiceAccountJWT("pod-name-doesnt-matter", fakeNamespace, fakeServiceAccount, clusterName)
	require.NoError(t, err)

	tempDir := t.TempDir()
	jwtPath := filepath.Join(tempDir, "token")
	require.NoError(t, os.WriteFile(jwtPath, []byte(jwt), 0600))
	require.NoError(t, os.Setenv(kubetoken.EnvVarCustomKubernetesTokenPath, jwtPath))

	// Test setup: craft a Terraform provider configuration
	terraformConfig := fmt.Sprintf(`
		provider "teleport" {
			addr = %q
			join_token = %q
			join_method = %q
			retry_base_duration = "900ms"
			retry_cap_duration = "4s"
			retry_max_tries = "12"
		}
	`, authHelper.ServerAddr(), testTokenName, types.JoinMethodKubernetes)

	terraformProvider := provider.New()
	terraformProviders := make(map[string]func() (tfprotov6.ProviderServer, error))
	terraformProviders["teleport"] = func() (tfprotov6.ProviderServer, error) {
		// Terraform configures provider on every test step, but does not clean up previous one, which produces
		// to "too many open files" at some point.
		//
		// With this statement we try to forcefully close previously opened client, which stays cached in
		// the provider variable.
		p, ok := terraformProvider.(*provider.Provider)
		require.True(t, ok)
		require.NoError(t, p.Close())
		return providerserver.NewProtocol6(terraformProvider)(), nil
	}

	// Test execution: apply a TF resource with the provider joining via MachineID
	dummyResource, err := fixtures.ReadFile(filepath.Join("fixtures", "app_0_create.tf"))
	require.NoError(t, err)
	testConfig := terraformConfig + "\n" + string(dummyResource)
	name := "teleport_app.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: testConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
		},
	})
}

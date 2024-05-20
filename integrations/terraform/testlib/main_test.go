/*
Copyright 2015-2021 Gravitational, Inc.

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
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/integrations/terraform/provider"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
)

//go:embed fixtures/*
var fixtures embed.FS

type TerraformBaseSuite struct {
	suite.Suite
	AuthHelper integration.AuthHelper
	// client represents plugin client
	client *client.Client
	// teleportConfig represents Teleport configuration
	teleportConfig lib.TeleportConfig
	// teleportFeatures represents enabled Teleport feature flags
	teleportFeatures *proto.Features
	// terraformConfig represents Terraform provider configuration
	terraformConfig string
	// terraformProvider represents an instance of a Terraform provider
	terraformProvider tfsdk.Provider
	// terraformProviders represents an array of provider factories that Terraform will use to instantiate the provider(s) under test.
	terraformProviders map[string]func() (tfprotov6.ProviderServer, error)
}

type TerraformSuiteOSSWithCache struct {
	TerraformBaseSuite
}
type TerraformSuiteOSS struct {
	TerraformBaseSuite
}
type TerraformSuiteEnterprise struct {
	TerraformBaseSuite
}
type TerraformSuiteEnterpriseWithCache struct {
	TerraformBaseSuite
}

func (s *TerraformBaseSuite) SetupSuite() {
	var err error
	t := s.T()

	os.Setenv("TF_ACC", "true")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s.client = s.AuthHelper.StartServer(t)

	pong, err := s.client.Ping(ctx)
	require.NoError(t, err)
	s.teleportFeatures = pong.GetServerFeatures()

	unrestricted := []string{"list", "create", "read", "update", "delete"}
	tfRole, err := types.NewRole("terraform", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: map[string]utils.Strings{"*": []string{"*"}},
			AppLabels:      map[string]utils.Strings{"*": []string{"*"}},
			NodeLabels:     map[string]utils.Strings{"*": []string{"*"}},
			Rules: []types.Rule{
				types.NewRule("token", unrestricted),
				types.NewRule("role", unrestricted),
				types.NewRule("user", unrestricted),
				types.NewRule("cluster_auth_preference", unrestricted),
				types.NewRule("cluster_networking_config", unrestricted),
				types.NewRule("cluster_maintenance_config", unrestricted),
				types.NewRule("session_recording_config", unrestricted),
				types.NewRule("db", unrestricted),
				types.NewRule("app", unrestricted),
				types.NewRule("github", unrestricted),
				types.NewRule("oidc", unrestricted),
				types.NewRule("okta_import_rule", unrestricted),
				types.NewRule("saml", unrestricted),
				types.NewRule("login_rule", unrestricted),
				types.NewRule("device", unrestricted),
				types.NewRule("access_list", unrestricted),
				types.NewRule("node", unrestricted),
				types.NewRule("bot", unrestricted),
			},
		},
	})
	require.NoError(t, err)

	tfRole, err = s.client.CreateRole(ctx, tfRole)
	require.NoError(t, err)

	tfUser, err := types.NewUser("terraform")
	require.NoError(t, err)
	tfUser.SetRoles([]string{tfRole.GetName()})
	tfUser, err = s.client.CreateUser(ctx, tfUser)
	require.NoError(t, err)

	// Sign an identity for the access plugin and generate its configuration
	s.teleportConfig.Addr = s.AuthHelper.ServerAddr()
	s.teleportConfig.Identity = s.AuthHelper.SignIdentityForUser(t, ctx, tfUser)

	s.terraformConfig = `
		provider "teleport" {
			addr = "` + s.teleportConfig.Addr + `"
			identity_file = file("` + s.teleportConfig.Identity + `")
			retry_base_duration = "900ms"
			retry_cap_duration = "4s"
			retry_max_tries = "12"
		}
	`
	// TLS creds are not used by regular config, but some tests check
	// how the provider works with TLS creds and direct auth connection
	credsPath := filepath.Join(t.TempDir(), "credentials")
	s.getTLSCreds(ctx, tfUser, credsPath)
	s.teleportConfig.ClientCrt = credsPath + ".crt"
	s.teleportConfig.ClientKey = credsPath + ".key"
	s.teleportConfig.RootCAs = credsPath + ".cas"

	s.terraformProvider = provider.New()
	s.terraformProviders = make(map[string]func() (tfprotov6.ProviderServer, error))
	s.terraformProviders["teleport"] = func() (tfprotov6.ProviderServer, error) {
		// Terraform configures provider on every test step, but does not clean up previous one, which produces
		// to "too many open files" at some point.
		//
		// With this statement we try to forcefully close previously opened client, which stays cached in
		// the provider variable.
		s.closeClient()
		return providerserver.NewProtocol6(s.terraformProvider)(), nil
	}
}

func (s *TerraformBaseSuite) getTLSCreds(ctx context.Context, user types.User, outputPath string) {
	key, err := libclient.GenerateRSAKey()
	require.NoError(s.T(), err)

	certs, err := s.client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: key.MarshalSSHPublicKey(),
		Username:  user.GetName(),
		Expires:   time.Now().Add(time.Hour),
		Format:    constants.CertificateFormatStandard,
	})
	require.NoError(s.T(), err)
	key.Cert = certs.SSH
	key.TLSCert = certs.TLS

	hostCAs, err := s.client.GetCertAuthorities(ctx, types.HostCA, false)
	require.NoError(s.T(), err)
	key.TrustedCerts = authclient.AuthoritiesToTrustedCerts(hostCAs)

	// write the cert+private key to the output:
	_, err = identityfile.Write(ctx, identityfile.WriteConfig{
		OutputPath:           outputPath,
		Key:                  key,
		Format:               identityfile.FormatTLS,
		OverwriteDestination: false,
		Writer:               &identityfile.StandardConfigWriter{},
	})
	require.NoError(s.T(), err)
}

func (s *TerraformBaseSuite) AfterTest(suiteName, testName string) {
	s.closeClient()
}

func (s *TerraformBaseSuite) SetupTest() {
}

func (s *TerraformBaseSuite) closeClient() {
	p, ok := s.terraformProvider.(*provider.Provider)
	require.True(s.T(), ok)
	if p != nil && p.Client != nil {
		require.NoError(s.T(), p.Client.Close())
	}
}

// getFixture loads fixture and returns it as string or <error> if failed
func (s *TerraformBaseSuite) getFixture(name string, formatArgs ...any) string {
	return s.getFixtureWithCustomConfig(name, s.terraformConfig, formatArgs...)
}

// getFixtureWithCustomConfig loads fixture and returns it as string or <error> if failed
func (s *TerraformBaseSuite) getFixtureWithCustomConfig(name, config string, formatArgs ...any) string {
	b, err := fixtures.ReadFile(filepath.Join("fixtures", name))
	if err != nil {
		return fmt.Sprintf("<error: %v fixture not found>", name)
	}
	return config + "\n" + fmt.Sprintf(string(b), formatArgs...)
}

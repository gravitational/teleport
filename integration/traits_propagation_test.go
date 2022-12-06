/*
Copyright 2022 Gravitational, Inc.

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

package integration

import (
	"context"
	"os/user"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"

	apiutils "github.com/gravitational/teleport/api/utils"
)

// TestTraitsPropagation makes sure that user traits are applied properly to
// roles in root and leaf clusters.
func TestTraitsPropagation(t *testing.T) {
	log := utils.NewLoggerForTests()

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	// Create root cluster.
	rc := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		Log:         log,
	})

	// Create leaf cluster.
	lc := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		Log:         log,
	})

	// Make root cluster config.
	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebService = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.SSH.Addr.Addr = rc.SSH
	rcConf.SSH.Labels = map[string]string{"env": "integration"}
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	// Make leaf cluster config.
	lcConf := service.MakeDefaultConfig()
	lcConf.DataDir = t.TempDir()
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebInterface = true
	lcConf.SSH.Enabled = true
	lcConf.SSH.Addr.Addr = lc.SSH
	lcConf.SSH.Labels = map[string]string{"env": "integration"}
	lcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	// Create identical user/role in both clusters.
	me, err := user.Current()
	require.NoError(t, err)

	role := services.NewImplicitRole()
	role.SetName("test")
	role.SetLogins(types.Allow, []string{me.Username})
	// Users created by CreateEx have "testing: integration" trait.
	role.SetNodeLabels(types.Allow, map[string]apiutils.Strings{"env": []string{"{{external.testing}}"}})

	rc.AddUserWithRole(me.Username, role)
	lc.AddUserWithRole(me.Username, role)

	// Establish trust b/w root and leaf.
	err = rc.CreateEx(t, lc.Secrets.AsSlice(), rcConf)
	require.NoError(t, err)
	err = lc.CreateEx(t, rc.Secrets.AsSlice(), lcConf)
	require.NoError(t, err)

	// Start both clusters.
	require.NoError(t, rc.Start())
	t.Cleanup(func() {
		rc.StopAll()
	})
	require.NoError(t, lc.Start())
	t.Cleanup(func() {
		lc.StopAll()
	})

	// Update root's certificate authority on leaf to configure role mapping.
	ca, err := lc.Process.GetAuthServer().GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.UserCA,
		DomainName: rc.Secrets.SiteName,
	}, false)
	require.NoError(t, err)
	ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
	ca.SetRoleMap(types.RoleMap{{Remote: role.GetName(), Local: []string{role.GetName()}}})
	err = lc.Process.GetAuthServer().UpsertCertAuthority(ca)
	require.NoError(t, err)

	// Run command in root.
	outputRoot, err := runCommand(t, rc, []string{"echo", "hello root"}, helpers.ClientConfig{
		Login:   me.Username,
		Cluster: "root.example.com",
		Host:    Loopback,
		Port:    helpers.Port(t, rc.SSH),
	}, 1)
	require.NoError(t, err)
	require.Equal(t, "hello root", strings.TrimSpace(outputRoot))

	// Run command in leaf.
	outputLeaf, err := runCommand(t, rc, []string{"echo", "hello leaf"}, helpers.ClientConfig{
		Login:   me.Username,
		Cluster: "leaf.example.com",
		Host:    Loopback,
		Port:    helpers.Port(t, lc.SSH),
	}, 1)
	require.NoError(t, err)
	require.Equal(t, "hello leaf", strings.TrimSpace(outputLeaf))
}

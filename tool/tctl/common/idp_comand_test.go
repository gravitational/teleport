/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
)

func TestIdPSAMLCommand(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

	t.Run("test_attribute_mapping", func(t *testing.T) {
		// Create user file
		userFilepath := filepath.Join(t.TempDir(), "user.yaml")
		require.NoError(t, os.WriteFile(userFilepath, []byte(user), 0644))

		// Create sp file
		spFilepath := filepath.Join(t.TempDir(), "sp.yaml")
		require.NoError(t, os.WriteFile(spFilepath, []byte(sp), 0644))

		// no --users argument
		err := runIdPSAMLCommand(t, fileConfig, []string{"saml", "test_attribute_mapping", "--sp", spFilepath})
		require.ErrorContains(t, err, "--users must be set")

		// non existent file should try to get user from cluster. Non-existent user error returns "user "name_of_user" not found" error.
		err = runIdPSAMLCommand(t, fileConfig, []string{"saml", "test_attribute_mapping", "--users", "no file", "--sp", spFilepath})
		require.ErrorContains(t, err, "not found")

		// empty user file
		require.NoError(t, os.WriteFile(userFilepath, []byte(""), 0644))
		err = runIdPSAMLCommand(t, fileConfig, []string{"saml", "test_attribute_mapping", "--users", userFilepath, "--sp", spFilepath})
		require.ErrorContains(t, err, "no users found in file")

		// empty sp file
		require.NoError(t, os.WriteFile(spFilepath, []byte(""), 0644))
		err = runIdPSAMLCommand(t, fileConfig, []string{"saml", "test_attribute_mapping", "--users", userFilepath, "--sp", spFilepath})
		require.ErrorContains(t, err, "empty service provider file")

		// no --sp argument
		err = runIdPSAMLCommand(t, fileConfig, []string{"saml", "test_attribute_mapping", "--users", userFilepath})
		require.ErrorContains(t, err, "--sp must be set")

		// no user and sp file.
		err = runIdPSAMLCommand(t, fileConfig, []string{"saml", "test_attribute_mapping"})
		require.ErrorContains(t, err, "no attributes to test")
	})
}

const user = `kind: user
metadata:
  name: testuser
spec:
  roles:
    - access
    - editor
  traits:
    aws_role_arns: null
    firstname:
      - test
    lastname:
      - tester
    groups:
      - testgroup
version: v2`

const sp = `kind: saml_idp_service_provider
version: v1
metadata:
   name: testapp
spec:
  entity_id: https://example.com/saml/metadata
  audience_uri: https://example.com/saml/acs
  attribute_mapping:
  - name: firstname
    value: user.spec.traits.firstname
  - name: lastname
    value: user.spec.traits.lastname
  - name: groups
    value: user.spec.traits.groups`

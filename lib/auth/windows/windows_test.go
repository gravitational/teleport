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

package windows

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func TestCRLDN(t *testing.T) {
	for _, test := range []struct {
		name        string
		clusterName string
		crlDN       string
		caType      types.CertAuthType
	}{
		{
			name:        "test cluster name",
			clusterName: "test",
			crlDN:       "CN=test,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "full cluster name",
			clusterName: "cluster.goteleport.com",
			crlDN:       "CN=cluster.goteleport.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "database CA",
			clusterName: "cluster.goteleport.com",
			caType:      types.DatabaseClientCA,
			crlDN:       "CN=cluster.goteleport.com,CN=TeleportDB,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "user CA",
			clusterName: "cluster.goteleport.com",
			caType:      types.UserCA,
			crlDN:       "CN=cluster.goteleport.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.crlDN, CRLDN(test.clusterName, "test.goteleport.com", test.caType))
		})
	}
}

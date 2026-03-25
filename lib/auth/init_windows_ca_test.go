// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package auth_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// Acquired from a local install, cluster name "zarquon2".
const exampleActiveKeys = `
{
  "ssh": [
    {
      "private_key": "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1DNENBUUF3QlFZREsyVndCQ0lFSUxwY0dFM3UwZlA1TkZWMGliVlZCdXpDMjlVVDVUd0ViMGRoYWErVnlPcloKLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLQo=",
      "public_key": "c3NoLWVkMjU1MTkgQUFBQUMzTnphQzFsWkRJMU5URTVBQUFBSUIzZzN2dHFMbHRPSDdyV3lQdDU3Sjc2VURCWnZNYVBiYWk1UlJmZHp2MjgK"
    }
  ],
  "tls": [
    {
      "cert": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUI4VENDQVplZ0F3SUJBZ0lSQU1sU2IrdjVGeE1iODU0Q3VaU0w1N0l3Q2dZSUtvWkl6ajBFQXdJd1dERVIKTUE4R0ExVUVDaE1JZW1GeWNYVnZiakl4RVRBUEJnTlZCQU1UQ0hwaGNuRjFiMjR5TVRBd0xnWURWUVFGRXljeQpOamMyTURJNE5qVTFNemd6TkRFNE1EVTBPRFkyT0RZNU1EWTVNek0xT1RNeU16YzBNall3SGhjTk1qVXhNVEEwCk1qQTFNREUxV2hjTk16VXhNVEF5TWpBMU1ERTFXakJZTVJFd0R3WURWUVFLRXdoNllYSnhkVzl1TWpFUk1BOEcKQTFVRUF4TUllbUZ5Y1hWdmJqSXhNREF1QmdOVkJBVVRKekkyTnpZd01qZzJOVFV6T0RNME1UZ3dOVFE0TmpZNApOamt3Tmprek16VTVNekl6TnpReU5qQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJBSWlWYk4wCnF5QXcrM0xKUEZOYldISnJvTmNPOFlMMUloaHdYRHc1TEN4MGl1MDFNTGs0dkZuUjlIS0FvblNnU2NGT1BiZisKM2VMZEhxcmkvaUc2QUFXalFqQkFNQTRHQTFVZER3RUIvd1FFQXdJQmhqQVBCZ05WSFJNQkFmOEVCVEFEQVFILwpNQjBHQTFVZERnUVdCQlRSZWhGMThPa2daamI5Yk55cGh1b1ZXQlM2N2pBS0JnZ3Foa2pPUFFRREFnTklBREJGCkFpQjFiaGwzTS9VdkdJaTg0Qk83dWt1OUZDZkswbCt6VVJYOWdHNUlsQjRsQ3dJaEFMaEFyVmZ1bTlKR2ZLcFgKUGw2R1gycXoyUjVLaHdvRjBwZ0RCUysxeFk1SQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==",
      "crl": "MIIBEjCBuAIBATAKBggqhkjOPQQDAjBYMREwDwYDVQQKEwh6YXJxdW9uMjERMA8GA1UEAxMIemFycXVvbjIxMDAuBgNVBAUTJzI2NzYwMjg2NTUzODM0MTgwNTQ4NjY4NjkwNjkzMzU5MzIzNzQyNhcNMjUxMTA0MjA0OTE1WhcNMzUxMTAyMjA1MDE1WqAvMC0wHwYDVR0jBBgwFoAU0XoRdfDpIGY2/WzcqYbqFVgUuu4wCgYDVR0UBAMCAQEwCgYIKoZIzj0EAwIDSQAwRgIhAKE3zRCSPblEKBtTdHpuqLTjeatZ4eKIbqZDpYh/+6mcAiEAimOkxNa4RdBSeSTIvvFOHWCpKKDSskuQWs8pNmnVS5U=",
      "key": "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JR0hBZ0VBTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEJHMHdhd0lCQVFRZ3BkK1NYWEdxNjRyU2RPd3YKL2pjMWdlVWJPUERuajNUZFh1ZlpLMGU1SFB1aFJBTkNBQVFDSWxXemRLc2dNUHR5eVR4VFcxaHlhNkRYRHZHQwo5U0lZY0Z3OE9Td3NkSXJ0TlRDNU9MeFowZlJ5Z0tKMG9FbkJUajIzL3QzaTNSNnE0djRodWdBRgotLS0tLUVORCBQUklWQVRFIEtFWS0tLS0tCg=="
    }
  ]
}`

func TestMigrateWindowsCA(t *testing.T) {
	t.Parallel()

	t.Run("new cluster", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		env := newMigrateCAEnv(t)

		require.NoError(t,
			auth.MigrateWindowsCA(ctx, env.Params),
			"migrateWindowsCA",
		)

		// Verify no new CAs created.
		trust := env.Params.Trust
		winCAs, err := trust.GetCertAuthorities(ctx, types.WindowsCA, false /* loadSigningKeys */)
		require.NoError(t, err)
		assert.Empty(t, winCAs, "Found unexpected WindowsCA entities")
		userCAs, err := trust.GetCertAuthorities(ctx, types.UserCA, false /* loadSigningKeys */)
		require.NoError(t, err)
		assert.Empty(t, userCAs, "Found unexpected UserCA entities")
	})

	t.Run("existing cluster", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		env := newMigrateCAEnv(t)
		trust := env.Params.Trust

		userID := types.CertAuthID{
			Type:       types.UserCA,
			DomainName: env.ClusterName,
		}
		winID := types.CertAuthID{
			Type:       types.WindowsCA,
			DomainName: env.ClusterName,
		}

		activeKeys := &types.CAKeySet{}
		require.NoError(t, json.Unmarshal([]byte(exampleActiveKeys), activeKeys))

		// Create the UserCA to migrate from.
		userCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        types.UserCA,
			ClusterName: env.ClusterName,
			ActiveKeys:  *activeKeys,
		})
		require.NoError(t, err)
		require.NoError(t, trust.CreateCertAuthority(ctx, userCA))
		// Read back so we get the latest revision.
		userCA, err = trust.GetCertAuthority(ctx, userID, true /* loadSigningKeys */)
		require.NoError(t, err)

		// Run migration.
		require.NoError(t,
			auth.MigrateWindowsCA(ctx, env.Params),
			"migrateWindowsCA",
		)

		// Verify the new WindowsCA.
		winCA, err := trust.GetCertAuthority(ctx, winID, true /* loadSigningKeys */)
		require.NoError(t, err)

		wantCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        types.WindowsCA,
			ClusterName: env.ClusterName,
			ActiveKeys: types.CAKeySet{
				TLS: activeKeys.TLS,
			},
		})
		require.NoError(t, err)
		wantCA.SetRevision(winCA.GetRevision())

		if diff := cmp.Diff(wantCA, winCA); diff != "" {
			t.Errorf("WindowsCA mismatch (-want +got)\n%s", diff)
		}

		t.Run("migration idempotent", func(t *testing.T) {
			// Migration runs without error.
			require.NoError(t, auth.MigrateWindowsCA(ctx, env.Params))

			// UserCA not modified.
			gotUser, err := trust.GetCertAuthority(ctx, userID, true /* loadSigningKeys */)
			require.NoError(t, err)
			if diff := cmp.Diff(userCA, gotUser); diff != "" {
				t.Errorf("UserCA mismatch (-want +got)\n%s", diff)
			}

			// WindowsCA not modified.
			gotWin, err := trust.GetCertAuthority(ctx, winID, true /* loadSigningKeys */)
			require.NoError(t, err)
			if diff := cmp.Diff(winCA, gotWin); diff != "" {
				t.Errorf("WindowsCA mismatch (-want +got)\n%s", diff)
			}
		})
	})
}

type migrateCAEnv struct {
	ClusterName string
	Params      auth.MigrateWindowsCAParams
}

func newMigrateCAEnv(t *testing.T) *migrateCAEnv {
	backend, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, backend.Close())
	})

	clusterConfiguration, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)

	// Assign a cluster name.
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "zarquon2", // must match exampleActiveKeys
		ClusterID:   "607e6238-3bcc-4169-b129-2a5b04e8c338",
	})
	require.NoError(t, err)
	require.NoError(t, clusterConfiguration.SetClusterName(cn))

	return &migrateCAEnv{
		ClusterName: cn.GetClusterName(),
		Params: auth.MigrateWindowsCAParams{
			Logger:               logtest.NewLogger(),
			ClusterConfiguration: clusterConfiguration,
			Trust:                local.NewCAService(backend),
		},
	}
}

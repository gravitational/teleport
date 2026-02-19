// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package storage

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/testing/protocmp"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

func TestRDPLicense(t *testing.T) {
	ctx := context.Background()
	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)
	storage := ProcessStorage{
		BackendStorage: mem,
		stateStorage:   mem,
	}

	_, err = storage.ReadRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   1,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	})
	require.True(t, trace.IsNotFound(err))

	licenseData := []byte{1, 2, 3}
	err = storage.WriteRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   1,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	}, licenseData)
	require.NoError(t, err)

	_, err = storage.ReadRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   2,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	})
	require.True(t, trace.IsNotFound(err))

	license, err := storage.ReadRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   1,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	})
	require.NoError(t, err)
	require.Equal(t, licenseData, license)
}

func Test_readOrGenerateHostID(t *testing.T) {
	id := uuid.New().String()
	const hostUUIDKey = "/host_uuid"
	type args struct {
		kubeBackend   *fakeKubeBackend
		hostIDContent string
		identity      []*state.Identity
	}
	tests := []struct {
		name             string
		args             args
		wantFunc         func(string) bool
		wantKubeItemFunc func(*backend.Item) bool
	}{
		{
			name: "load from storage without kube backend",
			args: args{
				kubeBackend:   nil,
				hostIDContent: id,
			},
			wantFunc: func(receivedID string) bool {
				return receivedID == id
			},
		},
		{
			name: "Kube Backend is available with key. Load from kube storage",
			args: args{
				kubeBackend: &fakeKubeBackend{
					getData: &backend.Item{
						Key:   backend.KeyFromString(hostUUIDKey),
						Value: []byte(id),
					},
					getErr: nil,
				},
			},
			wantFunc: func(receivedID string) bool {
				return receivedID == id
			},
			wantKubeItemFunc: func(i *backend.Item) bool {
				return i == nil
			},
		},
		{
			name: "No hostID available. Generate one and store it into Kube and Local Storage",
			args: args{
				kubeBackend: &fakeKubeBackend{
					getData: nil,
					getErr:  fmt.Errorf("key not found"),
				},
			},
			wantFunc: func(receivedID string) bool {
				_, err := uuid.Parse(receivedID)
				return err == nil
			},
			wantKubeItemFunc: func(i *backend.Item) bool {
				_, err := uuid.Parse(string(i.Value))
				return err == nil && i.Key.String() == hostUUIDKey
			},
		},
		{
			name: "No hostID available. Generate one and store it into Local Storage",
			args: args{
				kubeBackend: nil,
			},
			wantFunc: func(receivedID string) bool {
				_, err := uuid.Parse(receivedID)
				return err == nil
			},
			wantKubeItemFunc: nil,
		},
		{
			name: "No hostID available. Grab from provided static identity",
			args: args{
				kubeBackend: &fakeKubeBackend{
					getData: nil,
					getErr:  fmt.Errorf("key not found"),
				},

				identity: []*state.Identity{
					{
						ID: state.IdentityID{
							HostUUID: id,
						},
					},
				},
			},
			wantFunc: func(receivedID string) bool {
				return receivedID == id
			},
			wantKubeItemFunc: func(i *backend.Item) bool {
				_, err := uuid.Parse(string(i.Value))
				return err == nil && i.Key.String() == hostUUIDKey
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			// write host_uuid file to temp dir.
			if len(tt.args.hostIDContent) > 0 {
				err := hostid.WriteFile(dataDir, tt.args.hostIDContent)
				require.NoError(t, err)
			}

			cfg := &servicecfg.Config{
				DataDir:    dataDir,
				Logger:     slog.Default(),
				JoinMethod: types.JoinMethodToken,
				Identities: tt.args.identity,
			}

			var kubeBackend stateBackend
			if tt.args.kubeBackend != nil {
				kubeBackend = tt.args.kubeBackend
			}

			hostID, err := readOrGenerateHostID(context.Background(), cfg, kubeBackend)
			require.NoError(t, err)

			require.True(t, tt.wantFunc(hostID))

			if tt.args.kubeBackend != nil {
				require.True(t, tt.wantKubeItemFunc(tt.args.kubeBackend.putData))
			}
		})
	}
}

func genIdentity(t *testing.T, identityName string, immutableLabels *joiningv1.ImmutableLabels) *state.Identity {
	t.Helper()

	const clusterName = "example.com"

	a, err := testauthority.NewKeygen(modules.BuildOSS, time.Now)
	require.NoError(t, err)

	priv, pub, err := a.GenerateKeyPair()
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)
	tlsSigner, err := keys.ParsePrivateKey(priv)
	require.NoError(t, err)

	cert, err := testauthority.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        identityName,
		NodeName:      "test-node",
		TTL:           0,
		Identity: sshca.Identity{
			ClusterName: clusterName,
			SystemRole:  types.RoleNode,
		},
	})
	require.NoError(t, err)

	tlsCABytes, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Signer: tlsSigner,
		Entity: pkix.Name{
			CommonName:   clusterName,
			Organization: []string{clusterName},
		},
		TTL: defaults.CATTL,
	})
	require.NoError(t, err)

	ident, err := state.ReadSSHIdentityFromKeyPair(priv, cert)
	require.NoError(t, err)
	ident.SSHCACertBytes = [][]byte{priv}
	ident.TLSCACertsBytes = [][]byte{tlsCABytes}

	tlsCA, err := tlsca.FromCertAndSigner(tlsCABytes, tlsSigner)
	require.NoError(t, err)

	tlsIdent := tlsca.Identity{
		Username: identityName,
		Groups:   []string{string(types.RoleNode)},
	}
	subject, err := tlsIdent.Subject()
	require.NoError(t, err)
	tlsCert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: tlsSigner.Public(),
		Subject:   subject,
		NotAfter:  time.Now().UTC().Add(defaults.CATTL),
	})
	require.NoError(t, err)
	ident.TLSCertBytes = tlsCert
	ident.ImmutableLabels = immutableLabels

	return ident
}

func TestImmutableLabels(t *testing.T) {
	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)
	storage := ProcessStorage{
		BackendStorage: mem,
		stateStorage:   mem,
	}

	const identName = "testname"
	immutableLabels := &joiningv1.ImmutableLabels{
		Ssh: map[string]string{
			"foo": "bar",
		},
	}

	ident := genIdentity(t, identName, immutableLabels)
	ident.ImmutableLabels = immutableLabels

	// run in a loop to ensure we can overwrite existing labels
	for i := range 2 {
		err = storage.WriteIdentity(identName, *ident)
		require.NoError(t, err)
		ident, err = storage.ReadIdentity(identName, types.RoleNode)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(immutableLabels, ident.ImmutableLabels, protocmp.Transform()))
		immutableLabels.Ssh[fmt.Sprintf("label-%d", i)] = fmt.Sprintf("value-%d", i)
		ident.ImmutableLabels = immutableLabels
	}
}

type fakeKubeBackend struct {
	putData *backend.Item
	getData *backend.Item
	getErr  error
}

func (f *fakeKubeBackend) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	f.putData = &i
	return &backend.Lease{}, nil
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (f *fakeKubeBackend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	f.putData = &i
	return &backend.Lease{}, nil
}

// Get returns a single item or not found error
func (f *fakeKubeBackend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	return f.getData, f.getErr
}

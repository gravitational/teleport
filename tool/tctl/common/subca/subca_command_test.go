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

package subca_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	libsubca "github.com/gravitational/teleport/lib/subca"
	"github.com/gravitational/teleport/tool/tctl/common/subca"
)

func TestCommand_CreateOverrideCSR(t *testing.T) {
	t.Parallel()

	// Not consts so we can take their address.
	oLlama := "Llama"
	cnLlamo := "Llamo"

	// Variables for the "output to files" test.
	tempDir := t.TempDir()
	// Suffixes are calculated from the CAType and PEM public key, therefore
	// deterministic.
	wantWindows1File := filepath.Join(tempDir, "windows-f4522365-csr.pem")
	wantWindows2File := filepath.Join(tempDir, "windows-11b52b51-csr.pem")

	tests := []struct {
		name       string
		flags      []string                    // flags after "tctl auth create-override-csr"
		csrPEMs    []string                    // as returned by the server
		csrWarns   []*subcav1.CreateCSRWarning // as returned by the server
		wantReq    *subcav1.CreateCSRRequest
		wantStdout string
		wantStderr string
		wantFiles  map[string]string // filepath to content
	}{
		{
			name: "ok: db-client",
			flags: []string{
				"--type", "db-client",
			},
			csrPEMs: []string{dbClientCSRPEM},
			wantReq: subcav1.CreateCSRRequest_builder{
				CaType: string(types.DatabaseClientCA),
			}.Build(),
			wantStdout: dbClientCSRPEM + "\n",
		},
		{
			name: "ok: db_client",
			flags: []string{
				"--type", string(types.DatabaseClientCA),
			},
			csrPEMs: []string{dbClientCSRPEM},
			wantReq: subcav1.CreateCSRRequest_builder{
				CaType: string(types.DatabaseClientCA),
			}.Build(),
			wantStdout: dbClientCSRPEM + "\n",
		},
		{
			name: "ok: windows",
			flags: []string{
				"--type", string(types.WindowsCA),
			},
			csrPEMs: []string{windowsCSRPEM},
			wantReq: subcav1.CreateCSRRequest_builder{
				CaType: string(types.WindowsCA),
			}.Build(),
			wantStdout: windowsCSRPEM + "\n",
		},
		{
			name: "--public-key and --subject",
			flags: []string{
				"--type", string(types.WindowsCA),
				"--public-key=f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"--subject=O=Llama,CN=Llamo",
			},
			csrPEMs: []string{windowsCustomCSRPEM},
			wantReq: subcav1.CreateCSRRequest_builder{
				CaType: string(types.WindowsCA),
				PublicKeyHash: subcav1.PublicKeyHash_builder{
					Value: "f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				}.Build(),
				CustomSubject: subcav1.DistinguishedName_builder{
					Names: []*subcav1.AttributeTypeAndValue{
						subcav1.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 10}, Value: &oLlama}.Build(), // O
						subcav1.AttributeTypeAndValue_builder{Oid: []int32{2, 5, 4, 3}, Value: &cnLlamo}.Build(), // CN
					},
				}.Build(),
			}.Build(),
			wantStdout: windowsCustomCSRPEM + "\n",
		},
		{
			name: "multiple PEMs",
			flags: []string{
				"--type", string(types.WindowsCA),
			},
			csrPEMs: []string{
				windowsCSRPEM,
				windowsCSRPEM2,
			},
			wantReq: subcav1.CreateCSRRequest_builder{
				CaType: string(types.WindowsCA),
			}.Build(),
			wantStdout: windowsCSRPEM + "\n" + windowsCSRPEM2 + "\n",
		},
		{
			name: "output to files",
			flags: []string{
				"--type", string(types.WindowsCA),
				"--out", tempDir + "/",
			},
			csrPEMs: []string{
				windowsCSRPEM,
				windowsCSRPEM2,
			},
			wantReq: subcav1.CreateCSRRequest_builder{
				CaType: string(types.WindowsCA),
			}.Build(),
			wantStdout: "" +
				"Wrote " + wantWindows1File + "\n" +
				"Wrote " + wantWindows2File + "\n",
			wantFiles: map[string]string{
				wantWindows1File: windowsCSRPEM,
				wantWindows2File: windowsCSRPEM2,
			},
		},
		{
			name: "local only",
			flags: []string{
				"--type", string(types.WindowsCA),
				"--local-only",
			},
			csrPEMs: []string{
				windowsCSRPEM,
			},
			// Simulates "windowsCSRPEM2" not being accessible.
			csrWarns: []*subcav1.CreateCSRWarning{
				subcav1.CreateCSRWarning_builder{
					UserMessage:   "private key inaccessible",
					PublicKeyHash: "11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c",
				}.Build(),
			},
			wantReq: subcav1.CreateCSRRequest_builder{
				CaType:    string(types.WindowsCA),
				LocalOnly: true,
			}.Build(),
			wantStdout: windowsCSRPEM + "\n",
			wantStderr: `public key "11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c": private key inaccessible
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := &fakeAuthClient{
				csrPEMs:  test.csrPEMs,
				csrWarns: test.csrWarns,
			}
			clientFunc := func(ctx context.Context) (_ subca.SubCAClientSource, closeFn func(context.Context), _ error) {
				return fakeClient, func(ctx context.Context) {}, nil
			}

			env := newCommand(t)

			flags := append([]string{"auth", "create-override-csr"}, test.flags...)
			selectedCommand, err := env.App.Parse(flags)
			require.NoError(t, err, "app.Parse()")

			match, err := env.Cmd.TryRun(t.Context(), selectedCommand, clientFunc)
			assert.True(t, match)
			require.NoError(t, err, "Command errored")

			// Verify server request.
			if diff := cmp.Diff(test.wantReq, fakeClient.lastCSRRequest, protocmp.Transform()); diff != "" {
				t.Errorf("CreateCSRRequest mismatch (-want +got)\n%s", diff)
			}

			// Verify stdout/stderr.
			assert.Equal(t, test.wantStdout, env.Stdout.String(), "stdout mismatch")
			assert.Equal(t, test.wantStderr, env.Stderr.String(), "stderr mismatch")

			// Verify side effects.
			for filePath, wantContent := range test.wantFiles {
				val, err := os.ReadFile(filePath)
				if !assert.NoError(t, err, "Read file %s", filePath) {
					continue
				}
				assert.Equal(t, wantContent, string(val), "File %s", filePath)
			}
		})
	}
}

func TestFindMinHashes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in, want []string
	}{
		{
			name: "single hash",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
			},
			want: []string{
				"f4522365",
			},
		},
		{
			name: "multiple hashes",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c",
			},
			want: []string{
				"f4522365",
				"11b52b51",
			},
		},
		{
			name: "conflicts",
			in: []string{
				"bananallama11111",
				"bananallama21111",
				"bananallama31111",
			},
			want: []string{
				"bananallama1",
				"bananallama2",
				"bananallama3",
			},
		},
		{
			name: "duplicate hashes",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c",
			},
			want: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c",
			},
		},
		{
			name: "uneven/small hashes",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc", // normal len
				"aaaaaaa",       // <8 characters, aka <minLen.
				"bananallama1a", // clashes below.
				"bananallama2a", // clashes above.
			},
			want: []string{
				"f4522365888f", // trimmed
				"aaaaaaa",      // original (<minLen)
				"bananallama1", // trimmed
				"bananallama2", // trimmed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := subca.FindMinHashes(test.in)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("FindMinHashes mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestMakeCATypeNames(t *testing.T) {
	got := subca.MakeCATypeNames()
	want := []string{
		"db-client",
		"windows",
	}
	assert.Equal(t, want, got.Names, ""+
		"CAType names mismatch. "+
		"This is likely because a new entry was added to subca.SupportedCATypes(). "+
		"Verify the newly added name and, if working as intended, add it to want list above.",
	)
}

func TestCommand_CreateOverride(t *testing.T) {
	t.Parallel()

	// Write input certificates to files.
	// Add trailing spaces to some files for testing purposes.
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "cert.pem")
	chain0File := filepath.Join(tempDir, "chain0.pem")
	chain1File := filepath.Join(tempDir, "chain1.pem")
	require.NoError(t, os.WriteFile(certFile, []byte(overridePEM+"  \n\n  "), 0644))
	require.NoError(t, os.WriteFile(chain0File, []byte(chain0PEM+"  \n\n  "), 0644))
	require.NoError(t, os.WriteFile(chain1File, []byte(chain1PEM), 0644))

	var certPubKey string
	{
		cert, err := tlsutils.ParseCertificatePEM([]byte(overridePEM))
		require.NoError(t, err)
		certPubKey = libsubca.HashCertificatePublicKey(cert)
	}

	caID := subcav1.CertAuthorityOverrideID_builder{
		CaType: string(types.DatabaseClientCA),
	}.Build()

	tests := []struct {
		name    string
		flags   []string // flags after "tctl auth create-override"
		wantReq *subcav1.AddCertificateOverrideRequest
	}{
		{
			name: "ok",
			flags: []string{
				"--type", "db-client",
				certFile,
			},
			wantReq: subcav1.AddCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey:   certPubKey,
					Certificate: overridePEM,
				}.Build(),
			}.Build(),
		},
		{
			name: "use --disabled",
			flags: []string{
				"--type", "db-client",
				"--disabled",
				certFile,
			},
			wantReq: subcav1.AddCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey:   certPubKey,
					Certificate: overridePEM,
					Disabled:    true,
				}.Build(),
			}.Build(),
		},
		{
			name: "use --force",
			flags: []string{
				"--type", "db-client",
				"--force",
				certFile,
			},
			wantReq: subcav1.AddCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey:   certPubKey,
					Certificate: overridePEM,
				}.Build(),
				ForceImmediateDisable: true,
			}.Build(),
		},
		{
			name: "chain",
			flags: []string{
				"--type", "db-client",
				certFile,
				chain0File,
				chain1File,
			},
			wantReq: subcav1.AddCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey:   certPubKey,
					Certificate: overridePEM,
					Chain: []string{
						chain0PEM,
						chain1PEM,
					},
				}.Build(),
			}.Build(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := &fakeAuthClient{}
			clientFunc := func(ctx context.Context) (_ subca.SubCAClientSource, closeFn func(context.Context), _ error) {
				return fakeClient, func(ctx context.Context) {}, nil
			}

			env := newCommand(t)

			flags := append([]string{"auth", "create-override"}, test.flags...)
			selectedCommand, err := env.App.Parse(flags)
			require.NoError(t, err, "app.Parse()")

			match, err := env.Cmd.TryRun(t.Context(), selectedCommand, clientFunc)
			assert.True(t, match)
			require.NoError(t, err, "Command errored")

			// Verify server request.
			if diff := cmp.Diff(test.wantReq, fakeClient.lastAddCertificateRequest, protocmp.Transform()); diff != "" {
				t.Errorf("AddCertificateOverrideRequest mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestCommand_UpdateOverride(t *testing.T) {
	t.Parallel()

	// Write input certificates to files.
	// Add trailing spaces to some files for testing purposes.
	tempDir := t.TempDir()
	certAltFile := filepath.Join(tempDir, "cert-alt.pem")
	chain0File := filepath.Join(tempDir, "chain0.pem")
	chain1File := filepath.Join(tempDir, "chain1.pem")
	require.NoError(t, os.WriteFile(certAltFile, []byte(overrideAltPEM+"  \n\n  "), 0644))
	require.NoError(t, os.WriteFile(chain0File, []byte(chain0PEM+"  \n\n  "), 0644))
	require.NoError(t, os.WriteFile(chain1File, []byte(chain1PEM), 0644))

	var certPubKey string
	{
		cert, err := tlsutils.ParseCertificatePEM([]byte(overridePEM))
		require.NoError(t, err)
		certPubKey = libsubca.HashCertificatePublicKey(cert)
	}

	caID := subcav1.CertAuthorityOverrideID_builder{
		CaType: string(types.DatabaseClientCA),
	}.Build()

	newFieldMask := func(t *testing.T, paths ...string) *fieldmaskpb.FieldMask {
		m, err := fieldmaskpb.New(&subcav1.CertificateOverride{}, paths...)
		require.NoError(t, err)
		return m
	}

	tests := []struct {
		name    string
		flags   []string // flags after "tctl auth update-override"
		wantReq *subcav1.UpdateCertificateOverrideRequest
	}{
		{
			name: "enable override",
			flags: []string{
				"--type", "db-client",
				"--public-key", certPubKey,
				"--set-disabled", "false",
			},
			wantReq: subcav1.UpdateCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey: certPubKey,
				}.Build(),
				UpdateMask: newFieldMask(t, "disabled", "public_key"),
			}.Build(),
		},
		{
			name: "force disable override",
			flags: []string{
				"--type", "db-client",
				"--public-key", certPubKey,
				"--set-disabled", "true",
				"--force",
			},
			wantReq: subcav1.UpdateCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey: certPubKey,
					Disabled:  true,
				}.Build(),
				ForceImmediateDisable: true,
				UpdateMask:            newFieldMask(t, "disabled", "public_key"),
			}.Build(),
		},
		{
			name: "normalizes public key",
			flags: []string{
				"--type", "db-client",
				"--public-key", strings.ToUpper(certPubKey),
				"--set-disabled", "false",
			},
			wantReq: subcav1.UpdateCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey: certPubKey,
				}.Build(),
				UpdateMask: newFieldMask(t, "disabled", "public_key"),
			}.Build(),
		},
		{
			name: "update certificate",
			flags: []string{
				"--type", "db-client",
				"--set-cert", certAltFile,
			},
			wantReq: subcav1.UpdateCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey:   certPubKey,
					Certificate: overrideAltPEM,
				}.Build(),
				UpdateMask: newFieldMask(t, "certificate", "public_key"),
			}.Build(),
		},
		{
			name: "update certificate and chain",
			flags: []string{
				"--type", "db-client",
				"--set-cert", certAltFile,
				"--set-chain", chain0File,
				"--set-chain", chain1File,
			},
			wantReq: subcav1.UpdateCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey:   certPubKey,
					Certificate: overrideAltPEM,
					Chain: []string{
						chain0PEM,
						chain1PEM,
					},
				}.Build(),
				UpdateMask: newFieldMask(t, "certificate", "chain", "public_key"),
			}.Build(),
		},
		{
			name: "clear chain",
			flags: []string{
				"--type", "db-client",
				"--public-key", certPubKey,
				"--clear-chain",
			},
			wantReq: subcav1.UpdateCertificateOverrideRequest_builder{
				CaId: caID,
				CertificateOverride: subcav1.CertificateOverride_builder{
					PublicKey: certPubKey,
				}.Build(),
				UpdateMask: newFieldMask(t, "chain", "public_key"),
			}.Build(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := &fakeAuthClient{}
			clientFunc := func(ctx context.Context) (_ subca.SubCAClientSource, closeFn func(context.Context), _ error) {
				return fakeClient, func(ctx context.Context) {}, nil
			}

			env := newCommand(t)

			flags := append([]string{"auth", "update-override"}, test.flags...)
			selectedCommand, err := env.App.Parse(flags)
			require.NoError(t, err, "app.Parse()")

			match, err := env.Cmd.TryRun(t.Context(), selectedCommand, clientFunc)
			assert.True(t, match)
			require.NoError(t, err, "Command errored")

			// Verify server request.
			if diff := cmp.Diff(test.wantReq, fakeClient.lastUpdateCertificateRequest, protocmp.Transform()); diff != "" {
				t.Errorf("UpdateCertificateOverrideRequest mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestCommand_DeleteOverride(t *testing.T) {
	t.Parallel()

	var certPubKey string
	{
		cert, err := tlsutils.ParseCertificatePEM([]byte(overridePEM))
		require.NoError(t, err)
		certPubKey = libsubca.HashCertificatePublicKey(cert)
	}

	coID := subcav1.CertificateOverrideID_builder{
		CaType: string(types.DatabaseClientCA),
		PublicKeyHash: subcav1.PublicKeyHash_builder{
			Value: certPubKey,
		}.Build(),
	}.Build()

	tests := []struct {
		name    string
		flags   []string // flags after "tctl auth delete-override"
		wantReq *subcav1.RemoveCertificateOverrideRequest
	}{
		{
			name: "ok",
			flags: []string{
				"--type", "db-client",
				"--public-key", certPubKey,
			},
			wantReq: subcav1.RemoveCertificateOverrideRequest_builder{
				CertificateOverrideId: coID,
			}.Build(),
		},
		{
			name: "use --force",
			flags: []string{
				"--type", "db-client",
				"--public-key", certPubKey,
				"--force",
			},
			wantReq: subcav1.RemoveCertificateOverrideRequest_builder{
				CertificateOverrideId: coID,
				ForceImmediateDelete:  true,
			}.Build(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeClient := &fakeAuthClient{}
			clientFunc := func(ctx context.Context) (_ subca.SubCAClientSource, closeFn func(context.Context), _ error) {
				return fakeClient, func(ctx context.Context) {}, nil
			}

			env := newCommand(t)

			flags := append([]string{"auth", "delete-override"}, test.flags...)
			selectedCommand, err := env.App.Parse(flags)
			require.NoError(t, err, "app.Parse()")

			match, err := env.Cmd.TryRun(t.Context(), selectedCommand, clientFunc)
			assert.True(t, match)
			require.NoError(t, err, "Command errored")

			// Verify server request.
			if diff := cmp.Diff(test.wantReq, fakeClient.lastRemoveCertificateRequest, protocmp.Transform()); diff != "" {
				t.Errorf("RemoveCertificateOverrideRequest mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

// CSR PEMs for testing.
// The simplest way to get these is to run the "tctl auth create-override-csr"
// command.
// Starting a CA rotation in the "init" phase will make the corresponding CA
// generate multiple CSRs as well.
const (
	dbClientCSRPEM = `-----BEGIN CERTIFICATE REQUEST-----
MIICazCCAVMCAQAwJjERMA8GA1UEChMIemFycXVvbjIxETAPBgNVBAMTCHphcnF1
b24yMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArK67OiXGXaqorqAM
HRrUswRwfUMYJLq+wWTneao+KMhsvuWQPVrHmIkNpC1QjbDuLDTrc2ce/VIatMBZ
bJl6/hPZRJWOtuSbtI5syoGtXX1xbyVMOPMh4UW7+acn0VK3s3nwcVDYic33K6J+
SGXFUr92laNgyf49RQ0CUyDSs6H+xPyMr+oucuPdWTIkH9aYgXpd5bIFE3YzvEMM
mTNcstMTI9pvpC3h2Qw8uvBLnu5w+RW0V0qrmUbetX7RapYzXeTUC9EnQ5WFwsoH
gavsXr85qn3zaWa4D4PliPE9iNyaGN6k5b2UCASCkrVyYyDaiLnxPfpnF/MiQxkC
5MNBKwIDAQABoAAwDQYJKoZIhvcNAQELBQADggEBACz9u6+/8xpFc29P3m37cAol
tncq0dsapqP1NyQuYGlylutNZPHmFsDziXbr9e4L46qUNaoFmw7O0s8mvcmZdjzi
+4HorjyThtvzrjjXr/ul+JeDzXE592LcfA4EgoMnwhvz0Kp/YhMp+HHuK7gu6g3O
cQVvGtshNTVdUNfeLnBh0BRZ9JFQ1sLQIMsRqPSC9nA6lRwWuWt43m46EmjGrdWc
EZK5otyv3uhAMoIrLn2jrztz+cOVGXzZy27hnrM6MM1oKMlM4rbntX6TveG8fNoH
Q/IMZTusP3T17YRFoW/ov3+ERDheI6DxJxOcBhMd8/rW5FBJQVSnHZXVpemppxE=
-----END CERTIFICATE REQUEST-----`

	windowsCSRPEM = `-----BEGIN CERTIFICATE REQUEST-----
MIHgMIGIAgEAMCYxETAPBgNVBAoTCHphcnF1b24yMREwDwYDVQQDEwh6YXJxdW9u
MjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABAkIXaUNv8uoI9ICDijS4ciUOkSI
fSV9o3nslxvseZcswjzIE1fpiVPGwLzke7hfd/UcCzemuLoajOe3hXO12i6gADAK
BggqhkjOPQQDAgNHADBEAiByeQeUIJ2JXYYTaeSlODXHKbXlPh0XtiLj7v7rH5ZD
QgIgKT3/AuvpTuu2FIioZnx8feSWmPVDDPVO5cNaybkEhQM=
-----END CERTIFICATE REQUEST-----`

	windowsCSRPEM2 = `-----BEGIN CERTIFICATE REQUEST-----
MIHhMIGIAgEAMCYxETAPBgNVBAoTCHphcnF1b24yMREwDwYDVQQDEwh6YXJxdW9u
MjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABO9DT3fpxLGp9YJq1BCoqpszLH53
pW3AKK1VOhkEaZnTCV6Uvd92EibsP4HKto0NmfddL9JJcZ/BdLBN+9ESJxGgADAK
BggqhkjOPQQDAgNIADBFAiEAqpQsBWTJp7zmDwsDX4YW9Sw70o45BwWh3v1eZ1s9
JkUCIBw904bcK8yUxXqcHlpErnVN11e+Z435w6cwwO44MikG
-----END CERTIFICATE REQUEST-----`

	// O=Llama,CN=Llamo
	windowsCustomCSRPEM = `-----BEGIN CERTIFICATE REQUEST-----
MIHvMIGXAgEAMDUxDjAMBgNVBAoTBUxsYW1hMQ4wDAYDVQQDEwVMbGFtbzETMBEG
BSvODwQBEwh6YXJxdW9uMjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABAkIXaUN
v8uoI9ICDijS4ciUOkSIfSV9o3nslxvseZcswjzIE1fpiVPGwLzke7hfd/UcCzem
uLoajOe3hXO12i6gADAKBggqhkjOPQQDAgNHADBEAiBP+b9QDG4vb6ONy3g4iljg
cxvqL75ol8ta2P/m9elRDQIgX1OP0544EUSS2AlGxEotBqe7ZFg+HBsrzqCkQe7h
cxI=
-----END CERTIFICATE REQUEST-----`
)

// Certificate PEMs for testing.
const (
	// overridePEM is a CA override certificate.
	//
	// subject=O=zarquon2, CN=zarquon2
	// issuer=O=Llama Corp, OU=Llama CA, CN=Llama Database CA
	overridePEM = `-----BEGIN CERTIFICATE-----
MIICmjCCAkCgAwIBAgIUNvMJUfqbjR1o7ID+GlCEVfvADF4wCgYIKoZIzj0EAwIw
RDETMBEGA1UEChMKTGxhbWEgQ29ycDERMA8GA1UECxMITGxhbWEgQ0ExGjAYBgNV
BAMTEUxsYW1hIERhdGFiYXNlIENBMB4XDTI2MDYxMDE2MzUwMFoXDTMxMDYwOTE2
MzUwMFowJjERMA8GA1UEChMIemFycXVvbjIxETAPBgNVBAMTCHphcnF1b24yMIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAm2wHhbEzHddPlg8RJxyeAArV
Ku+jMLGWSVjSVH5Pn2301cSe28KC5iaRuYfzf6sdEftElyV7fUdeZ+i07QTWYuMT
3B8x5J62n/g3R4RQxYY4ivNaISnApO435cPYDvDIpAzZ3qBKRdvXDAsc0/bjiG39
qIi04V29WRz1IjS/BPWyRP6hoeP6gbtl4Z2YE4y1MwGvl3a0L4wEFvWrhZrhlpKu
3IzryaE/lqAaE6H4zRGNL7qPUAxxnr4n1YdDN/0BGY7OYOvea9BPseHHzIAXo/v3
5T1pn7aVhso4pkgb5Jj715LYw7uWlXV3y/f0R4Y2BP73SqkRhRzvMgQL+1DRBQID
AQABo2MwYTAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4E
FgQUYsc7XihR9bNRm8UdGZ1dlRZGVUQwHwYDVR0jBBgwFoAUDC1GIwweTul8uBzi
Q4FKcCFBuhowCgYIKoZIzj0EAwIDSAAwRQIgebCxPjXntz2l3XtVzPYa63ayFwNX
42EJAwazCrdS26sCIQC5koQINAk8I1+j3y3PmSw27DzTByWtnYtk9PI7xz9blQ==
-----END CERTIFICATE-----`

	// overrideAltPEM is an alternative CA override certificate.
	//
	// subject=O=zarquon2, CN=zarquon2
	// issuer=O=Llama Corp, OU=Llama CA, CN=Llama Database CA
	overrideAltPEM = `-----BEGIN CERTIFICATE-----
MIICmjCCAkCgAwIBAgIUQ5YwT7MX5bPnQ2v9QV33WPG3DeEwCgYIKoZIzj0EAwIw
RDETMBEGA1UEChMKTGxhbWEgQ29ycDERMA8GA1UECxMITGxhbWEgQ0ExGjAYBgNV
BAMTEUxsYW1hIERhdGFiYXNlIENBMB4XDTI2MDYxNzIxMzAwMFoXDTMxMDYxNjIx
MzAwMFowJjERMA8GA1UEChMIemFycXVvbjIxETAPBgNVBAMTCHphcnF1b24yMIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAm2wHhbEzHddPlg8RJxyeAArV
Ku+jMLGWSVjSVH5Pn2301cSe28KC5iaRuYfzf6sdEftElyV7fUdeZ+i07QTWYuMT
3B8x5J62n/g3R4RQxYY4ivNaISnApO435cPYDvDIpAzZ3qBKRdvXDAsc0/bjiG39
qIi04V29WRz1IjS/BPWyRP6hoeP6gbtl4Z2YE4y1MwGvl3a0L4wEFvWrhZrhlpKu
3IzryaE/lqAaE6H4zRGNL7qPUAxxnr4n1YdDN/0BGY7OYOvea9BPseHHzIAXo/v3
5T1pn7aVhso4pkgb5Jj715LYw7uWlXV3y/f0R4Y2BP73SqkRhRzvMgQL+1DRBQID
AQABo2MwYTAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4E
FgQUYsc7XihR9bNRm8UdGZ1dlRZGVUQwHwYDVR0jBBgwFoAUDC1GIwweTul8uBzi
Q4FKcCFBuhowCgYIKoZIzj0EAwIDSAAwRQIgN9ch2UET9HJGkUt47c/7UCAv9yyh
fO01mshT7mnA+vICIQCh0ep22NPhvvZ08MpGLKcfuN6umluM3fYuxNPk8GtG5A==
-----END CERTIFICATE-----`

	// chain0PEM is an external (non-Teleport) intermediate CA.
	//
	// subject=O=Llama Corp, OU=Llama CA, CN=Llama Database CA
	// issuer=O=Llama Corp, OU=Llama CA, CN=Llama Root CA
	chain0PEM = `-----BEGIN CERTIFICATE-----
MIIB7DCCAZKgAwIBAgIUZeRuKuCUiD7KiNfNp6Gg5vDb5nowCgYIKoZIzj0EAwIw
QDETMBEGA1UEChMKTGxhbWEgQ29ycDERMA8GA1UECxMITGxhbWEgQ0ExFjAUBgNV
BAMTDUxsYW1hIFJvb3QgQ0EwHhcNMjYwNTA3MTk0NjAwWhcNMzEwNTA2MTk0NjAw
WjBEMRMwEQYDVQQKEwpMbGFtYSBDb3JwMREwDwYDVQQLEwhMbGFtYSBDQTEaMBgG
A1UEAxMRTGxhbWEgRGF0YWJhc2UgQ0EwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNC
AAQf57W1nn+Tbgi0VHurKbPlhIQP9nA1TnM79SLevTlxnNCQXsCI/vUhAcyhdZbs
O58AEuUijPK7k7+egL8AdlUmo2YwZDAOBgNVHQ8BAf8EBAMCAQYwEgYDVR0TAQH/
BAgwBgEB/wIBATAdBgNVHQ4EFgQUDC1GIwweTul8uBziQ4FKcCFBuhowHwYDVR0j
BBgwFoAUc4PZtEUN3muyF188EO3IJ2CPZdQwCgYIKoZIzj0EAwIDSAAwRQIhAJmN
S5xRejN6jfAJsJG1pkyg8GD05sfyhe65DhjyvawOAiAuS+595Kfk16K0L+ngkRMb
tktSPZQPhhfZHqhU+9F3uw==
-----END CERTIFICATE-----`

	// chain1PEM is an external (non-Teleport) self-signed root CA.
	//
	// subject=O=Llama Corp, OU=Llama CA, CN=Llama Root CA
	chain1PEM = `-----BEGIN CERTIFICATE-----
MIIBxDCCAWqgAwIBAgIUScZfsbAZPKpWwCnCn/98vsNJw30wCgYIKoZIzj0EAwIw
QDETMBEGA1UEChMKTGxhbWEgQ29ycDERMA8GA1UECxMITGxhbWEgQ0ExFjAUBgNV
BAMTDUxsYW1hIFJvb3QgQ0EwHhcNMjYwNDAxMjAxMjAwWhcNMzEwMzMxMjAxMjAw
WjBAMRMwEQYDVQQKEwpMbGFtYSBDb3JwMREwDwYDVQQLEwhMbGFtYSBDQTEWMBQG
A1UEAxMNTGxhbWEgUm9vdCBDQTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPk4
01BUTEnyJjj6ao3hVwJPiTfdyhAdK6ZnBLUO4J5bapHd7qgO0UfRMWCPAprynBuW
4kQpit787juRVtOLaQyjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBRzg9m0RQ3ea7IXXzwQ7cgnYI9l1DAKBggqhkjOPQQDAgNI
ADBFAiBtpWrwZUPUFZTmSdlYX12oS0ZCy+sY0f7OtXHusCMo3gIhAM0c86jXYpq9
gT/2q6LYL8NGcL6ParNRimdwhm+0+xOe
-----END CERTIFICATE-----`
)

type fakeAuthClient struct {
	subcav1.SubCAServiceClient

	csrPEMs                      []string
	csrWarns                     []*subcav1.CreateCSRWarning
	lastCSRRequest               *subcav1.CreateCSRRequest
	lastAddCertificateRequest    *subcav1.AddCertificateOverrideRequest
	lastUpdateCertificateRequest *subcav1.UpdateCertificateOverrideRequest
	lastRemoveCertificateRequest *subcav1.RemoveCertificateOverrideRequest
}

func (f *fakeAuthClient) SubCAClient() subcav1.SubCAServiceClient {
	return f
}

func (f *fakeAuthClient) CreateCSR(
	ctx context.Context, req *subcav1.CreateCSRRequest, _ ...grpc.CallOption) (*subcav1.CreateCSRResponse, error) {
	f.lastCSRRequest = req

	if len(f.csrPEMs) == 0 {
		return nil, trace.BadParameter("cannot create CSRs for CA")
	}

	resp := subcav1.CreateCSRResponse_builder{
		Csrs:     make([]*subcav1.CertificateSigningRequest, len(f.csrPEMs)),
		Warnings: f.csrWarns,
	}.Build()
	for i, pem := range f.csrPEMs {
		resp.GetCsrs()[i] = subcav1.CertificateSigningRequest_builder{
			Pem: pem,
		}.Build()
	}
	return resp, nil
}

func (f *fakeAuthClient) AddCertificateOverride(
	ctx context.Context,
	req *subcav1.AddCertificateOverrideRequest,
	opts ...grpc.CallOption,
) (*subcav1.AddCertificateOverrideResponse, error) {
	f.lastAddCertificateRequest = req
	return &subcav1.AddCertificateOverrideResponse{}, nil
}

func (f *fakeAuthClient) UpdateCertificateOverride(
	ctx context.Context,
	req *subcav1.UpdateCertificateOverrideRequest,
	opts ...grpc.CallOption,
) (*subcav1.UpdateCertificateOverrideResponse, error) {
	f.lastUpdateCertificateRequest = req
	return &subcav1.UpdateCertificateOverrideResponse{}, nil
}

func (f *fakeAuthClient) RemoveCertificateOverride(
	ctx context.Context,
	req *subcav1.RemoveCertificateOverrideRequest,
	opts ...grpc.CallOption,
) (*subcav1.RemoveCertificateOverrideResponse, error) {
	f.lastRemoveCertificateRequest = req
	return &subcav1.RemoveCertificateOverrideResponse{}, nil
}

func TestCommand_PubKeyHash(t *testing.T) {
	t.Parallel()

	const certPEM = `-----BEGIN CERTIFICATE-----
MIIERzCCAq+gAwIBAgIRAMZcaomNE+VygiNyxoSwzr8wDQYJKoZIhvcNAQELBQAw
fTEeMBwGA1UEChMVbWtjZXJ0IGRldmVsb3BtZW50IENBMSkwJwYDVQQLDCBhbGFu
QHphcnF1b24yLmxvY2FsIChBbGFuIFBhcnJhKTEwMC4GA1UEAwwnbWtjZXJ0IGFs
YW5AemFycXVvbjIubG9jYWwgKEFsYW4gUGFycmEpMB4XDTI2MDMzMDE0NTI0OVoX
DTI4MDYzMDE0NTI0OVowSTEnMCUGA1UEChMebWtjZXJ0IGRldmVsb3BtZW50IGNl
cnRpZmljYXRlMR4wHAYDVQQLDBVhbGFuQE1hYyAoQWxhbiBQYXJyYSkwggEiMA0G
CSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDb43vlSk68rJMK/kKjGEP06y81A4kt
e+sfwsGXbevwk0565FfXsN3oraUkb5QG0/Cv6pZotBMOx8g8Gbkb0Od4OoESD9l1
SQF1F+9DNyrGQscLg9cz+qVaG9q0DIwGMKNmchZwZES0PGeA6l6CAgsR06NoTht3
Cv8id6myHlOg+pKoil5iQ9kydxrU9qku4dRlYggbMynn9KaEm0i5g7ipIsnvnRkL
WOJup9qLPYdQIwQfKtQiJ6SG95pypnkxku4qbPsWKqReYH694HwWBo5XKERbFEX2
5lQNGpwWRLFxquZiheiu7C/jxxzxLnxnScGcERpI6om3plK1OgbRPx9RAgMBAAGj
djB0MA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAfBgNVHSME
GDAWgBRIeRs75N2PBLqa0s7ILgxvFho9PTAsBgNVHREEJTAjgglsb2NhbGhvc3SH
BH8AAAGHEAAAAAAAAAAAAAAAAAAAAAEwDQYJKoZIhvcNAQELBQADggGBAA52BvfF
Xs084HMVyRu09FYwu9fiHfD3Whyq7khPhBhF9/r+/VBEjie5oi4Vk9z/OVWPSp37
XN+UeNvAHQh07ntAW+7m/Gfrt4lX7kK46+Gjgzu8YPBNhAkYEhJz8ViJshSSfRnu
itwRn/9J7qVzGud6Hhn+Zc+CBrIe467jT+iUoA1iiZDUjZHqo1O14SwomuB2/CzC
t4C4ZQSxU6yOEcvJP6C0hwSZYKht5+WGwLy95cvX9kk+RrqEysLatbUfr1xrE35y
2VEVbb8ppWMrd6njbJjGXBO1bb1kNUgj0GYI+0oJeu5W4AXBYS97tAntNWFcfeFS
O/sfRlbBpQNYxA045fO/Dc3qKhZAVfLLWL/mK3XSRS8gz0iBESlDXDVqChKeOzZV
Fv4gVOCfPBtGRcyntw5YMjRBr1M5E7lR7f0EiEf98XK+7qCyrgBa63/F1YConCCK
OCILEpSS/B19NPlXOYipYx7lPCoRHCsDQk/RuQ0doilw+Gx1f/lMMH3Z/g==
-----END CERTIFICATE-----`
	// Pre-calculated pub key hash of certPEM.
	const want = "0fa1e6306b74b68aca58636482dc3cb5ff64ba17dbff6d808cf901169745e6be\n"

	tests := []struct {
		name           string
		prepareCommand func(t *testing.T, stdin *bytes.Buffer) []string
	}{
		{
			name: "--cert=file",
			prepareCommand: func(t *testing.T, stdin *bytes.Buffer) []string {
				tmp := t.TempDir()
				name := tmp + "/cert.pem"
				require.NoError(t,
					os.WriteFile(name, []byte(certPEM), 0644),
					"Write test certificate",
				)
				// "tctl auth pub-key-hash --cert=cert.pem"
				return []string{"auth", "pub-key-hash", "--cert", name}
			},
		},
		{
			name: "--cert='-' (stdin)",
			prepareCommand: func(t *testing.T, stdin *bytes.Buffer) []string {
				stdin.WriteString(certPEM)
				// "tctl auth pub-key-hash --cert='-'"
				return []string{"auth", "pub-key-hash", "--cert=-"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			env := newCommand(t)

			// Prepare/parse flags.
			flags := test.prepareCommand(t, env.Stdin)
			selectedCommand, err := env.App.Parse(flags)
			require.NoError(t, err, "app.Parse()")

			// Run.
			match, err := env.Cmd.TryRun(t.Context(), selectedCommand, nil)
			require.True(t, match, "TryRun() returned match=false")
			require.NoError(t, err, "TryRun() errored")

			// Verify output.
			got := env.Stdout.String()
			assert.Equal(t, want, got, "pub-key-hash output mismatch")
		})
	}
}

type commandEnv struct {
	App                   *kingpin.Application
	Cmd                   *subca.Command
	Stdin, Stdout, Stderr *bytes.Buffer
}

func newCommand(_ *testing.T) *commandEnv {
	app := kingpin.New("tctl", "")
	authCmd := app.Command("auth", "")

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	subCACmd := &subca.Command{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	subCACmd.Initialize(authCmd, nil, nil)

	return &commandEnv{
		App:    app,
		Cmd:    subCACmd,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
}

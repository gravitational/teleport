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

package tlsca

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509/pkix"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/cryptopatch"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/fixtures"
)

// TestPrincipals makes sure that SAN extension of generated x509 cert gets
// correctly set with DNS names and IP addresses based on the provided
// principals.
func TestPrincipals(t *testing.T) {
	tests := []struct {
		name       string
		createFunc func() (*CertAuthority, error)
	}{
		{
			name: "FromKeys",
			createFunc: func() (*CertAuthority, error) {
				return FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
			},
		},
		{
			name: "FromCertAndSigner",
			createFunc: func() (*CertAuthority, error) {
				signer, err := keys.ParsePrivateKey([]byte(fixtures.TLSCAKeyPEM))
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return FromCertAndSigner([]byte(fixtures.TLSCACertPEM), signer)
			},
		},
		{
			name: "FromTLSCertificate",
			createFunc: func() (*CertAuthority, error) {
				cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return FromTLSCertificate(cert)
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ca, err := test.createFunc()
			require.NoError(t, err)

			privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
			require.NoError(t, err)

			hostnames := []string{"localhost", "example.com"}
			ips := []string{"127.0.0.1", "192.168.1.1"}

			clock := clockwork.NewFakeClock()

			certBytes, err := ca.GenerateCertificate(CertificateRequest{
				Clock:     clock,
				PublicKey: privateKey.Public(),
				Subject:   pkix.Name{CommonName: "test"},
				NotAfter:  clock.Now().Add(time.Hour),
				DNSNames:  append(hostnames, ips...),
			})
			require.NoError(t, err)

			cert, err := ParseCertificatePEM(certBytes)
			require.NoError(t, err)
			require.ElementsMatch(t, cert.DNSNames, hostnames)
			var certIPs []string
			for _, ip := range cert.IPAddresses {
				certIPs = append(certIPs, ip.String())
			}
			require.ElementsMatch(t, certIPs, ips)
		})
	}
}

func TestRenewableIdentity(t *testing.T) {
	clock := clockwork.NewFakeClock()
	expires := clock.Now().Add(1 * time.Hour)

	ca, err := FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	identity := Identity{
		Username:  "alice@example.com",
		Groups:    []string{"admin"},
		Expires:   expires,
		Renewable: true,
	}

	subj, err := identity.Subject()
	require.NoError(t, err)
	require.NotNil(t, subj)

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)

	parsed, err := FromSubject(cert.Subject, expires)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.True(t, parsed.Renewable)
}

func TestJoinAttributes(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	expires := clock.Now().Add(1 * time.Hour)

	ca, err := FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	identity := Identity{
		Username:      "bot-bernard",
		Groups:        []string{"bot-bernard"},
		BotName:       "bernard",
		BotInstanceID: "1234-5678",
		Expires:       expires,
		JoinAttributes: &workloadidentityv1pb.JoinAttrs{
			Kubernetes: &workloadidentityv1pb.JoinAttrsKubernetes{
				ServiceAccount: &workloadidentityv1pb.JoinAttrsKubernetesServiceAccount{
					Namespace: "default",
					Name:      "foo",
				},
				Pod: &workloadidentityv1pb.JoinAttrsKubernetesPod{
					Name: "bar",
				},
			},
		},
	}

	subj, err := identity.Subject()
	require.NoError(t, err)
	require.NotNil(t, subj)

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)

	parsed, err := FromSubject(cert.Subject, expires)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Empty(t, cmp.Diff(parsed, &identity, protocmp.Transform()))
}

// TestKubeExtensions test ASN1 subject kubernetes extensions
func TestKubeExtensions(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ca, err := FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	expires := clock.Now().Add(time.Hour)
	identity := Identity{
		Username:     "alice@example.com",
		Groups:       []string{"admin"},
		Impersonator: "bob@example.com",
		// Generate a certificate restricted for
		// use against a kubernetes endpoint, and not the API server endpoint
		// otherwise proxies can generate certs for any user.
		Usage:             []string{teleport.UsageKubeOnly},
		KubernetesGroups:  []string{"system:masters", "admin"},
		KubernetesUsers:   []string{"IAM#alice@example.com"},
		KubernetesCluster: "kube-cluster",
		TeleportCluster:   "tele-cluster",
		RouteToDatabase: RouteToDatabase{
			ServiceName: "postgres-rds",
			Protocol:    "postgres",
			Username:    "postgres",
		},
		DatabaseNames: []string{"postgres", "main"},
		DatabaseUsers: []string{"postgres", "alice"},
		Expires:       expires,
	}

	subj, err := identity.Subject()
	require.NoError(t, err)

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)
	out, err := FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err)
	require.False(t, out.Renewable)
	require.Empty(t, cmp.Diff(out, &identity, cmpopts.EquateApproxTime(time.Second)))
}

func TestDatabaseExtensions(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ca, err := FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	expires := clock.Now().Add(time.Hour)
	identity := Identity{
		Username:        "alice@example.com",
		Groups:          []string{"admin"},
		Impersonator:    "bob@example.com",
		Usage:           []string{teleport.UsageDatabaseOnly},
		TeleportCluster: "tele-cluster",
		RouteToDatabase: RouteToDatabase{
			ServiceName: "postgres-rds",
			Protocol:    "postgres",
			Username:    "postgres",
			Roles:       []string{"read_only"},
		},
		DatabaseNames: []string{"postgres", "main"},
		DatabaseUsers: []string{"postgres", "alice"},
		Expires:       expires,
	}

	subj, err := identity.Subject()
	require.NoError(t, err)

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)
	out, err := FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err)
	require.False(t, out.Renewable)
	require.Empty(t, cmp.Diff(out, &identity, cmpopts.EquateApproxTime(time.Second)))
}

func TestAzureExtensions(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ca, err := FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	expires := clock.Now().Add(time.Hour)
	identity := Identity{
		Username:        "alice@example.com",
		Groups:          []string{"admin"},
		Impersonator:    "bob@example.com",
		Usage:           []string{teleport.UsageAppsOnly},
		AzureIdentities: []string{"azure-identity-1", "azure-identity-2"},
		RouteToApp: RouteToApp{
			SessionID:     "43de4ffa8509aff3e3990e941400a403a12a6024d59897167b780ec0d03a1f15",
			ClusterName:   "teleport.example.com",
			Name:          "azure-app",
			AzureIdentity: "azure-identity-3",
		},
		TeleportCluster: "tele-cluster",
		Expires:         expires,
	}

	subj, err := identity.Subject()
	require.NoError(t, err)

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)
	out, err := FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, &identity, cmpopts.EquateApproxTime(time.Second)))
	require.Equal(t, "43de4ffa8509aff3e3990e941400a403a12a6024d59897167b780ec0d03a1f15", out.RouteToApp.SessionID)
}

func TestIdentity_ToFromSubject(t *testing.T) {
	assertStringOID := func(t *testing.T, want string, oid asn1.ObjectIdentifier, subj *pkix.Name, msgAndArgs ...any) {
		for _, en := range subj.ExtraNames {
			if !oid.Equal(en.Type) {
				continue
			}

			got, ok := en.Value.(string)
			require.True(t, ok, "Value for OID %v is not a string: %T", oid, en.Value)
			assert.Equal(t, want, got, msgAndArgs)
			return
		}
		t.Fatalf("OID %v not found", oid)
	}

	tests := []struct {
		name          string
		identity      *Identity
		assertSubject func(t *testing.T, identity *Identity, subj *pkix.Name)
	}{
		{
			name: "device extensions",
			identity: &Identity{
				Username: "llama",                      // Required.
				Groups:   []string{"editor", "viewer"}, // Required.
				DeviceExtensions: DeviceExtensions{
					DeviceID:     "deviceid1",
					AssetTag:     "assettag2",
					CredentialID: "credentialid3",
				},
			},
			assertSubject: func(t *testing.T, identity *Identity, subj *pkix.Name) {
				want := identity.DeviceExtensions
				assertStringOID(t, want.DeviceID, DeviceIDExtensionOID, subj, "DeviceID mismatch")
				assertStringOID(t, want.AssetTag, DeviceAssetTagExtensionOID, subj, "AssetTag mismatch")
				assertStringOID(t, want.CredentialID, DeviceCredentialIDExtensionOID, subj, "CredentialID mismatch")
			},
		},
		{
			name: "user type: sso",
			identity: &Identity{
				Username: "llama",                      // Required.
				Groups:   []string{"editor", "viewer"}, // Required.
				UserType: "sso",
			},
			assertSubject: func(t *testing.T, identity *Identity, subj *pkix.Name) {
				assertStringOID(t, string(identity.UserType), UserTypeASN1ExtensionOID, subj, "User Type mismatch")
			},
		},
		{
			name: "user type: local",
			identity: &Identity{
				Username: "llama",                      // Required.
				Groups:   []string{"editor", "viewer"}, // Required.
				UserType: "local",
			},
			assertSubject: func(t *testing.T, identity *Identity, subj *pkix.Name) {
				assertStringOID(t, string(identity.UserType), UserTypeASN1ExtensionOID, subj, "User Type mismatch")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identity := test.identity

			// Marshal identity into subject.
			subj, err := identity.Subject()
			require.NoError(t, err, "Subject failed")
			test.assertSubject(t, identity, &subj)

			// ExtraNames are appended to Names when the cert is created.
			subj.Names = append(subj.Names, subj.ExtraNames...)
			subj.ExtraNames = nil

			// Extract identity from subject and verify that no data got lost.
			got, err := FromSubject(subj, identity.Expires)
			require.NoError(t, err, "FromSubject failed")
			if diff := cmp.Diff(identity, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("FromSubject mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestGCPExtensions(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ca, err := FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	expires := clock.Now().Add(time.Hour)
	identity := Identity{
		Username:           "alice@example.com",
		Groups:             []string{"admin"},
		Impersonator:       "bob@example.com",
		Usage:              []string{teleport.UsageAppsOnly},
		GCPServiceAccounts: []string{"acct-1@example-123456.iam.gserviceaccount.com", "acct-2@example-123456.iam.gserviceaccount.com"},
		RouteToApp: RouteToApp{
			SessionID:         "43de4ffa8509aff3e3990e941400a403a12a6024d59897167b780ec0d03a1f15",
			ClusterName:       "teleport.example.com",
			Name:              "GCP-app",
			GCPServiceAccount: "acct-3@example-123456.iam.gserviceaccount.com",
		},
		TeleportCluster: "tele-cluster",
		Expires:         expires,
	}

	subj, err := identity.Subject()
	require.NoError(t, err)

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)
	out, err := FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, &identity, cmpopts.EquateApproxTime(time.Second)))
}

func TestIdentity_GetUserMetadata(t *testing.T) {
	tests := []struct {
		name     string
		identity Identity
		want     apievents.UserMetadata
	}{
		{
			name: "user metadata",
			identity: Identity{
				Username:     "alpaca",
				Impersonator: "llama",
				RouteToApp: RouteToApp{
					AWSRoleARN:        "awsrolearn",
					AzureIdentity:     "azureidentity",
					GCPServiceAccount: "gcpaccount",
				},
				ActiveRequests: []string{"accessreq1", "accessreq2"},
				BotName:        "",
			},
			want: apievents.UserMetadata{
				User:              "alpaca",
				Impersonator:      "llama",
				AWSRoleARN:        "awsrolearn",
				AccessRequests:    []string{"accessreq1", "accessreq2"},
				AzureIdentity:     "azureidentity",
				GCPServiceAccount: "gcpaccount",
				UserKind:          apievents.UserKind_USER_KIND_HUMAN,
			},
		},
		{
			name: "user metadata for bot",
			identity: Identity{
				Username:      "bot-alpaca",
				BotName:       "alpaca",
				BotInstanceID: "123-123",
			},
			want: apievents.UserMetadata{
				User:          "bot-alpaca",
				UserKind:      apievents.UserKind_USER_KIND_BOT,
				BotName:       "alpaca",
				BotInstanceID: "123-123",
			},
		},
		{
			name: "device metadata",
			identity: Identity{
				Username: "llama",
				DeviceExtensions: DeviceExtensions{
					DeviceID:     "deviceid1",
					AssetTag:     "assettag1",
					CredentialID: "credentialid1",
				},
				BotName: "",
			},
			want: apievents.UserMetadata{
				User: "llama",
				TrustedDevice: &apievents.DeviceMetadata{
					DeviceId:     "deviceid1",
					AssetTag:     "assettag1",
					CredentialId: "credentialid1",
				},
				UserKind: apievents.UserKind_USER_KIND_HUMAN,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.identity.GetUserMetadata()
			want := test.want
			if !proto.Equal(&got, &want) {
				t.Errorf("GetUserMetadata mismatch (-want +got)\n%s", cmp.Diff(want, got))
			}
		})
	}
}

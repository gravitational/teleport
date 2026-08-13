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

package testlib

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
)

const certAuthorityOverrideCAType = "db_client"

func (s *TerraformSuiteEnterprise) newOverrideCertificate(ctx context.Context) (certPEM, rootPEM string) {
	s.T().Helper()

	resp, err := s.client.SubCAClient().CreateCSR(ctx, &subcav1.CreateCSRRequest{
		CaType: certAuthorityOverrideCAType,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(resp.GetCsrs())

	block, _ := pem.Decode([]byte(resp.GetCsrs()[0].GetPem()))
	s.Require().NotNil(block, "CSR is not valid PEM")
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	s.Require().NoError(err)

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)

	now := time.Now()
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "External Test Root"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(30 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	s.Require().NoError(err)
	rootCert, err := x509.ParseCertificate(rootDER)
	s.Require().NoError(err)

	overrideTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		RawSubject:            csr.RawSubject,
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(15 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	overrideDER, err := x509.CreateCertificate(rand.Reader, overrideTemplate, rootCert, csr.PublicKey, rootKey)
	s.Require().NoError(err)

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: overrideDER}))
	rootPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}))
	return certPEM, rootPEM
}

func (s *TerraformSuiteEnterprise) getCertAuthorityOverride(ctx context.Context) (*subcav1.CertAuthorityOverride, error) {
	resp, err := s.client.SubCAClient().GetCertAuthorityOverride(ctx, &subcav1.GetCertAuthorityOverrideRequest{
		CaId: &subcav1.CertAuthorityOverrideID{
			CaType: certAuthorityOverrideCAType,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetCaOverride(), nil
}

func (s *TerraformSuiteEnterprise) deleteCertAuthorityOverride(ctx context.Context) {
	s.T().Helper()

	_, err := s.client.SubCAClient().DeleteCertAuthorityOverride(ctx, &subcav1.DeleteCertAuthorityOverrideRequest{
		CaId: &subcav1.CertAuthorityOverrideID{
			CaType: certAuthorityOverrideCAType,
		},
		ForceImmediateDelete: true,
	})
	if err != nil && !trace.IsNotFound(err) {
		s.Require().NoError(err)
	}
}

func (s *TerraformSuiteEnterprise) TestCertAuthorityOverride() {
	t := s.T()
	ctx := t.Context()

	clusterName, err := s.client.GetDomainName(ctx)
	s.Require().NoError(err)

	certPEM, rootPEM := s.newOverrideCertificate(ctx)

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.getCertAuthorityOverride(ctx)
		switch {
		case err == nil:
			return trace.Errorf("cert authority override was not deleted")
		case trace.IsNotFound(err):
			return nil
		default:
			return err
		}
	}

	name := "teleport_cert_authority_override.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("cert_authority_override_0_create.tf", clusterName, strings.TrimSpace(certPEM), strings.TrimSpace(rootPEM)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindCertAuthorityOverride),
					resource.TestCheckResourceAttr(name, "sub_kind", certAuthorityOverrideCAType),
					resource.TestCheckResourceAttr(name, "version", "v1"),
					resource.TestCheckResourceAttr(name, "metadata.name", clusterName),
					resource.TestCheckResourceAttr(name, "spec.certificate_overrides.0.certificate", certPEM),
					resource.TestCheckResourceAttr(name, "spec.certificate_overrides.0.chain.0", rootPEM),
					resource.TestCheckResourceAttr(name, "spec.certificate_overrides.0.disabled", "false"),
				),
			},
			{
				Config:   s.getFixture("cert_authority_override_0_create.tf", clusterName, strings.TrimSpace(certPEM), strings.TrimSpace(rootPEM)),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("cert_authority_override_1_update.tf", clusterName, strings.TrimSpace(certPEM), strings.TrimSpace(rootPEM)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "metadata.description", "updated by terraform test"),
					resource.TestCheckResourceAttr(name, "spec.certificate_overrides.0.disabled", "true"),
				),
			},
			{
				Config:   s.getFixture("cert_authority_override_1_update.tf", clusterName, strings.TrimSpace(certPEM), strings.TrimSpace(rootPEM)),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestCertAuthorityOverrideDataSource() {
	t := s.T()
	ctx := t.Context()

	clusterName, err := s.client.GetDomainName(ctx)
	s.Require().NoError(err)

	certPEM, rootPEM := s.newOverrideCertificate(ctx)

	_, err = s.client.SubCAClient().CreateCertAuthorityOverride(ctx, &subcav1.CreateCertAuthorityOverrideRequest{
		CaOverride: &subcav1.CertAuthorityOverride{
			Kind:     types.KindCertAuthorityOverride,
			SubKind:  certAuthorityOverrideCAType,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: clusterName},
			Spec: &subcav1.CertAuthorityOverrideSpec{
				CertificateOverrides: []*subcav1.CertificateOverride{
					{
						Certificate: certPEM,
						Chain:       []string{rootPEM},
					},
				},
			},
		},
	})
	s.Require().NoError(err)
	t.Cleanup(func() { s.deleteCertAuthorityOverride(context.Background()) })

	name := "data.teleport_cert_authority_override.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("cert_authority_override_data_source.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", types.KindCertAuthorityOverride),
					resource.TestCheckResourceAttr(name, "sub_kind", certAuthorityOverrideCAType),
					resource.TestCheckResourceAttr(name, "metadata.name", clusterName),
					resource.TestCheckResourceAttr(name, "spec.certificate_overrides.0.certificate", certPEM),
				),
			},
		},
	})
}

func (s *TerraformSuiteEnterprise) TestImportCertAuthorityOverride() {
	t := s.T()
	ctx := t.Context()

	clusterName, err := s.client.GetDomainName(ctx)
	s.Require().NoError(err)

	certPEM, rootPEM := s.newOverrideCertificate(ctx)

	r := "teleport_cert_authority_override"
	id := "test_import"
	name := r + "." + id

	_, err = s.client.SubCAClient().CreateCertAuthorityOverride(ctx, &subcav1.CreateCertAuthorityOverrideRequest{
		CaOverride: &subcav1.CertAuthorityOverride{
			Kind:     types.KindCertAuthorityOverride,
			SubKind:  certAuthorityOverrideCAType,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: clusterName},
			Spec: &subcav1.CertAuthorityOverrideSpec{
				CertificateOverrides: []*subcav1.CertificateOverride{
					{
						Certificate: certPEM,
						Chain:       []string{rootPEM},
					},
				},
			},
		},
	})
	s.Require().NoError(err)
	t.Cleanup(func() { s.deleteCertAuthorityOverride(context.Background()) })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		Steps: []resource.TestStep{
			{
				Config:        s.terraformConfig + "\n" + `resource "` + r + `" "` + id + `" { }`,
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: certAuthorityOverrideCAType,
				ImportStateCheck: func(state []*terraform.InstanceState) error {
					s.Require().Len(state, 1)
					s.Require().Equal(certAuthorityOverrideCAType, state[0].Attributes["sub_kind"])
					s.Require().Equal(clusterName, state[0].Attributes["metadata.name"])
					s.Require().Equal(certPEM, state[0].Attributes["spec.certificate_overrides.0.certificate"])
					return nil
				},
			},
		},
	})
}

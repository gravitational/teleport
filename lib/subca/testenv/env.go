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

package testenv

import (
	"cmp"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/subca"
	"github.com/gravitational/teleport/lib/tlscatest"
)

// EnvParams holds creational parameters for [Env].
type EnvParams struct {
	// Clock is a clock override.
	// Optional. If unset a real clock is used.
	Clock clockwork.Clock
	// ClusterName to use.
	// Optional. If unset a default is used.
	ClusterName string
	// SubCAParams allows setting params not managed by env.
	// Optional.
	SubCAParams *local.SubCAServiceParams
	// CATypesToCreate defines which CAs are created on env initialization.
	// If empty none are created.
	CATypesToCreate []types.CertAuthType
	// SkipExternalRoot, if true, skips creation of the external root CA.
	SkipExternalRoot bool
}

// Env is a complete Sub CA test environment.
type Env struct {
	// Clock is the env clock. Always non-nil.
	Clock clockwork.Clock
	// ClusterName used by the env. Always non-empty.
	ClusterName string

	Backend backend.Backend
	Trust   *local.CA
	SubCA   *local.SubCAService

	// ExternalRoot is an external root CA.
	// Created unless SkipExternalRoot is true.
	ExternalRoot *CA
}

// New creates new Env.
func New(t *testing.T, p EnvParams) *Env {
	t.Helper()

	clock := p.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	// Init backend.
	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mem.Close(), "Close memory backend")
	})

	// Init Trust service.
	trust := local.NewCAService(mem)

	// Init Sub CA service.
	var subCAParams local.SubCAServiceParams
	if p.SubCAParams != nil {
		subCAParams = *p.SubCAParams
	}
	subCAParams.Backend = mem
	subCA, err := local.NewSubCAService(subCAParams)
	require.NoError(t, err)

	const defaultClusterName = "zarquon"
	env := &Env{
		Clock:       clock,
		ClusterName: cmp.Or(p.ClusterName, defaultClusterName),
		Backend:     mem,
		Trust:       trust,
		SubCA:       subCA,
	}

	env.initCAs(t, p.CATypesToCreate)

	if !p.SkipExternalRoot {
		var err error
		env.ExternalRoot, err = NewSelfSignedCA(&CAParams{
			Clock: clock,
		})
		require.NoError(t, err)
	}

	return env
}

func (env *Env) initCAs(t *testing.T, caTypes []types.CertAuthType) {
	if len(caTypes) == 0 {
		return
	}
	for _, caType := range caTypes {
		env.createCA(t, caType)
	}
}

func (env *Env) createCA(t *testing.T, caType types.CertAuthType) {
	ctx := t.Context()

	keyPEM, certPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
		ClusterName: env.ClusterName,
	})
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: env.ClusterName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert:    certPEM,
					Key:     keyPEM,
					KeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, env.Trust.CreateCertAuthority(ctx, ca))
}

// NewOverrideForCAType queries a CA from Trust and creates a
// CertAuthorityOverride for it, as per [Env.NewOverrideForCA].
//
// The env.ExternalRoot is used to create the override.
func (env *Env) NewOverrideForCAType(
	t *testing.T,
	caType types.CertAuthType,
) *subcav1.CertAuthorityOverride {
	t.Helper()
	require.NotNil(t, env.ExternalRoot, "Env.ExternalRoot is nil")

	ca, err := env.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
		Type:       caType,
		DomainName: env.ClusterName,
	}, false /* loadSigningKeys */)
	require.NoError(t, err)

	return env.NewOverrideForCA(t, ca, env.ExternalRoot)
}

// NewOverrideForCA creates a CertAuthorityOverride resource for the given CA.
//
// All active keys are overridden with disabled overrides, as per
// [Env.NewDisabledCertificateOverride]. Additional keys are not overridden.
//
// If externalRoot is nil then Env.ExternalRoot is used.
func (env *Env) NewOverrideForCA(
	t *testing.T,
	ca types.CertAuthority,
	externalRoot *CA,
) *subcav1.CertAuthorityOverride {
	t.Helper()

	var overrides []*subcav1.CertificateOverride
	for _, kp := range ca.GetActiveKeys().TLS {
		cert, err := tlsutils.ParseCertificatePEM(kp.Cert)
		require.NoError(t, err)

		co := env.NewDisabledCertificateOverride(t, cert, externalRoot)
		overrides = append(overrides, co)
	}

	return &subcav1.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(ca.GetType()),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: ca.GetName(),
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{
			CertificateOverrides: overrides,
		},
	}
}

// NewDisabledCertificateOverride creates a disabled CertificateOverride for the
// given certificate.
//
// If externalRoot is nil then Env.ExternalRoot is used.
func (env *Env) NewDisabledCertificateOverride(
	t *testing.T,
	cert *x509.Certificate,
	externalRoot *CA,
) *subcav1.CertificateOverride {
	t.Helper()

	if externalRoot == nil {
		require.NotNil(t, env.ExternalRoot, "Env.ExternalRoot is nil")
		externalRoot = env.ExternalRoot
	}

	overrideCA, err := externalRoot.NewIntermediateCA(&CAParams{
		Clock: env.Clock,
		Pub:   cert.PublicKey,
		Template: &x509.Certificate{
			Subject: pkix.Name{
				Organization: cert.Subject.Organization, // ClusterName.
			},
			NotAfter: cert.NotAfter, // Cannot expire after the CA certificate.
		},
	})
	require.NoError(t, err)

	return &subcav1.CertificateOverride{
		PublicKey:   subca.HashCertificatePublicKey(overrideCA.Cert),
		Certificate: string(overrideCA.CertPEM),
		Disabled:    true,
	}
}

// MakeCAChain is a convenience wrapper over the standalone [MakeCAChain].
func (env *Env) MakeCAChain(t *testing.T, length int) CAChain {
	t.Helper()

	chain, err := MakeCAChain(length, &CAParams{
		Clock: env.Clock,
	})
	require.NoError(t, err)
	return chain
}

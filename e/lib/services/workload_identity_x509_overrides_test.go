package services

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestParseWorkloadIdentityX509IssuerOverride(t *testing.T) {
	t.Parallel()

	_, err := ParseWorkloadIdentityX509IssuerOverride(&workloadidentityv1.X509IssuerOverride{
		Kind: "something",
	})
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	_, err = ParseWorkloadIdentityX509IssuerOverride(&workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V2,
	})
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	_, err = ParseWorkloadIdentityX509IssuerOverride(&workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
	})
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	_, err = ParseWorkloadIdentityX509IssuerOverride(&workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "none",
		},
	})
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	_, err = ParseWorkloadIdentityX509IssuerOverride(&workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "notdefault",
		},
	})
	require.ErrorAs(t, err, new(*trace.BadParameterError))

	emptyOverride, err := ParseWorkloadIdentityX509IssuerOverride(&workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "default",
		},
	})
	require.NoError(t, err)
	require.Empty(t, emptyOverride.overrides)

	ca1 := selfSignedCA(t, "ca1")
	ca2 := selfSignedCA(t, "ca2")
	ca3 := selfSignedCA(t, "ca3")
	extCA := selfSignedCA(t, "extCA")
	xca1 := crossSignedCA(t, "xca1", ca1, extCA)
	xca2 := crossSignedCA(t, "xca2", ca2, extCA)

	override, err := ParseWorkloadIdentityX509IssuerOverride(&workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "default",
		},
		Spec: &workloadidentityv1.X509IssuerOverrideSpec{
			Overrides: []*workloadidentityv1.X509IssuerOverrideSpec_Override{
				{
					Issuer: xca1.Leaf.Raw,
					Chain: [][]byte{
						xca1.Leaf.Raw,
					},
				},
				{
					Issuer: xca2.Leaf.Raw,
					Chain: [][]byte{
						xca2.Leaf.Raw,
					},
				},
			},
		},
	})
	require.NoError(t, err)

	ca, chain, ok := override.GetCAOverride(&tlsca.CertAuthority{
		Cert:   ca1.Leaf,
		Signer: ca1.PrivateKey.(crypto.Signer),
	})
	require.True(t, ok)
	require.Equal(t, ca1.Leaf.RawSubjectPublicKeyInfo, ca.Cert.RawSubjectPublicKeyInfo)
	require.Equal(t, "xca1", ca.Cert.Subject.CommonName)
	require.Len(t, chain, 1)
	require.Equal(t, xca1.Leaf.Raw, chain[0])

	_, _, ok = override.GetCAOverride(&tlsca.CertAuthority{
		Cert:   ca3.Leaf,
		Signer: ca3.PrivateKey.(crypto.Signer),
	})
	require.False(t, ok)
}

func selfSignedCA(t *testing.T, cn string) tls.Certificate {
	sn, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 159))
	require.NoError(t, err)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		NotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),

		SerialNumber: sn,
		Subject: pkix.Name{
			CommonName: cn,
		},

		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},

		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}
}

func crossSignedCA(t *testing.T, cn string, old tls.Certificate, parent tls.Certificate) tls.Certificate {
	sn, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 159))
	require.NoError(t, err)

	template := &x509.Certificate{
		NotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),

		SerialNumber: sn,
		Subject: pkix.Name{
			CommonName: cn,
		},

		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},

		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent.Leaf, old.Leaf.PublicKey, parent.PrivateKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  old.PrivateKey,
		Leaf:        cert,
	}
}

func TestWorkloadIdentityX509IssuerOverrideCache(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	bk, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)
	defer bk.Close()

	storage, err := local.NewWorkloadIdentityX509OverridesService(bk)
	require.NoError(t, err)

	cacheClock := clockwork.NewFakeClock()
	c, err := newWorkloadIdentityX509IssuerOverrideCache(storage, bk, slog.Default(), ctx, cacheClock)
	require.NoError(t, err)

	blankCA := &tlsca.CertAuthority{Cert: new(x509.Certificate)}
	ca, _, err := c.GetWorkloadIdentityX509CAOverride(ctx, "", blankCA)
	require.NoError(t, err)
	require.Same(t, blankCA, ca)

	_, err = storage.CreateX509IssuerOverride(ctx, &workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "default",
		},
	})
	require.NoError(t, err)

	ca, _, err = c.GetWorkloadIdentityX509CAOverride(ctx, "", blankCA)
	require.NoError(t, err)
	require.Same(t, blankCA, ca)

	cacheClock.Advance(2 * time.Minute)

	_, _, err = c.GetWorkloadIdentityX509CAOverride(ctx, "", blankCA)
	require.ErrorAs(t, err, new(*trace.BadParameterError))
	require.ErrorContains(t, err, "\"default\" exists but is missing issuers")

	require.NoError(t, storage.DeleteX509IssuerOverride(ctx, "default"))

	go c.RunWatcher(ctx)
	// no longer advancing the clock, only relying on cache invalidation from the watcher

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		ca, _, err = c.GetWorkloadIdentityX509CAOverride(ctx, "", blankCA)
		assert.NoError(t, err)
		assert.Same(t, blankCA, ca)
	}, 5*time.Second, 50*time.Millisecond)

	_, err = storage.CreateX509IssuerOverride(ctx, &workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "default",
		},
	})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, _, err = c.GetWorkloadIdentityX509CAOverride(ctx, "", blankCA)
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
		assert.ErrorContains(t, err, "\"default\" exists but is missing issuers")
	}, 5*time.Second, 50*time.Millisecond)
}

func TestWorkloadIdentityX509IssuerOverridePrefix(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	bk, err := memory.New(memory.Config{
		Context:   ctx,
		EventsOff: true,
	})
	require.NoError(t, err)
	defer bk.Close()

	svc, err := local.NewWorkloadIdentityX509OverridesService(bk)
	require.NoError(t, err)

	_, err = bk.Get(ctx, backend.NewKey(workloadIdentityX509IssuerOverridePrefix, "default"))
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	_, err = svc.CreateX509IssuerOverride(ctx, &workloadidentityv1.X509IssuerOverride{
		Kind:    types.KindWorkloadIdentityX509IssuerOverride,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "default",
		},
	})
	require.NoError(t, err)

	_, err = bk.Get(ctx, backend.NewKey(workloadIdentityX509IssuerOverridePrefix, "default"))
	require.NoError(t, err)
}

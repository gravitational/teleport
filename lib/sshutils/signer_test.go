package sshutils

import (
	"io"

	"github.com/gravitational/teleport/lib/defaults"
	"golang.org/x/crypto/ssh"
	"gopkg.in/check.v1"
)

type AlgSignerSuite struct{}

var _ = check.Suite(&AlgSignerSuite{})

func (s *AlgSignerSuite) TestAlgSignerNoop(c *check.C) {
	sig := newMockSigner(ssh.KeyAlgoECDSA521)

	// ECDSA key should be returned as-is, not wrapped.
	c.Assert(AlgSigner(sig, defaults.CASignatureAlgorithm), check.Equals, sig)
}

func (s *AlgSignerSuite) TestAlgSigner(c *check.C) {
	for _, pubType := range []string{ssh.KeyAlgoRSA, ssh.CertAlgoRSAv01} {
		c.Logf("signer public key type: %q", pubType)
		rsaSigner := newMockSigner(pubType)

		wrapped := AlgSigner(rsaSigner, defaults.CASignatureAlgorithm)
		// RSA key or cert should get wrapped.
		c.Assert(wrapped, check.Not(check.Equals), rsaSigner)
		wrappedAS := wrapped.(ssh.AlgorithmSigner)

		// Simple Sign call should use the enforced SHA-2 algorithm.
		wrapped.Sign(nil, nil)
		c.Assert(rsaSigner.lastAlg, check.Equals, defaults.CASignatureAlgorithm)
		rsaSigner.lastAlg = ""

		// SignWithAlgorithm without specifying an algorithm should use the
		// enforced SHA-2 algorithm.
		wrappedAS.SignWithAlgorithm(nil, nil, "")
		c.Assert(rsaSigner.lastAlg, check.Equals, defaults.CASignatureAlgorithm)
		rsaSigner.lastAlg = ""

		// SignWithAlgorithm *with* an algorithm should respect the provided
		// algorithm.
		wrappedAS.SignWithAlgorithm(nil, nil, "foo")
		c.Assert(rsaSigner.lastAlg, check.Equals, "foo")
	}
}

type mockSigner struct {
	ssh.Signer
	lastAlg string
	pubType string
}

func newMockSigner(pubType string) *mockSigner {
	return &mockSigner{pubType: pubType}
}

func (s *mockSigner) PublicKey() ssh.PublicKey {
	return mockPublicKey{pubType: s.pubType}
}

func (s *mockSigner) SignWithAlgorithm(rand io.Reader, data []byte, alg string) (*ssh.Signature, error) {
	s.lastAlg = alg
	return nil, nil
}

type mockPublicKey struct {
	ssh.PublicKey
	pubType string
}

func (p mockPublicKey) Type() string {
	return p.pubType
}

package sshutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"io"

	"golang.org/x/crypto/ssh"
	"gopkg.in/check.v1"
)

type CompatSignerSuite struct{}

var _ = check.Suite(&CompatSignerSuite{})

func (s *CompatSignerSuite) TestCompatSignerNoop(c *check.C) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, check.IsNil)
	ecdsaSigner, err := ssh.NewSignerFromSigner(ecdsaKey)
	c.Assert(err, check.IsNil)

	// ECDSA key should be returned as-is, not wrapped.
	c.Assert(CompatSigner(ecdsaSigner), check.Equals, ecdsaSigner)
}

func (s *CompatSignerSuite) TestCompatSigner(c *check.C) {
	rsaSigner, err := newMockRSASigner()
	c.Assert(err, check.IsNil)

	wrapped := CompatSigner(rsaSigner)
	// RSA key should get wrapped.
	c.Assert(wrapped, check.Not(check.Equals), rsaSigner)
	wrappedAS := wrapped.(ssh.AlgorithmSigner)

	// Simple Sign call should use the enforced SHA-2 algorithm.
	wrapped.Sign(nil, nil)
	c.Assert(rsaSigner.lastAlg, check.Equals, ssh.SigAlgoRSASHA2512)
	rsaSigner.lastAlg = ""

	// SignWithAlgorithm without specifying an algorithm should use the
	// enforced SHA-2 algorithm.
	wrappedAS.SignWithAlgorithm(nil, nil, "")
	c.Assert(rsaSigner.lastAlg, check.Equals, ssh.SigAlgoRSASHA2512)
	rsaSigner.lastAlg = ""

	// SignWithAlgorithm *with* an algorithm should respect the provided
	// algorithm.
	wrappedAS.SignWithAlgorithm(nil, nil, "foo")
	c.Assert(rsaSigner.lastAlg, check.Equals, "foo")
}

type mockRSASigner struct {
	ssh.Signer
	lastAlg string
}

func newMockRSASigner() (*mockRSASigner, error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2056)
	if err != nil {
		return nil, err
	}
	rsaSigner, err := ssh.NewSignerFromSigner(rsaKey)
	if err != nil {
		return nil, err
	}
	return &mockRSASigner{Signer: rsaSigner}, nil
}

func (s *mockRSASigner) SignWithAlgorithm(rand io.Reader, data []byte, alg string) (*ssh.Signature, error) {
	s.lastAlg = alg
	return nil, nil
}

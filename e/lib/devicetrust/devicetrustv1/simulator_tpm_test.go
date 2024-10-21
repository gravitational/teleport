package devicetrustv1_test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

type ekCertGenerator func(ekKey crypto.PublicKey) ([]byte, error)

var ekCertSerial = "ff:ff:ff:ff"

func newFakeEKCertCA() (ekCertGenerator ekCertGenerator, caPEM []byte, err error) {
	// Create CA cert
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1337),
		Subject: pkix.Name{
			CommonName:   "EK Root CA 1",
			Organization: []string{"Really Great TPM Manufacturer Inc."},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, caKey.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}
	caPEMBuf := bytes.NewBuffer(nil)
	err = pem.Encode(caPEMBuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return nil, nil, err
	}

	ekCertGenerator = func(ekKey crypto.PublicKey) ([]byte, error) {
		serialInt := big.NewInt(0)
		serialInt, ok := serialInt.SetString(
			strings.ReplaceAll(ekCertSerial, ":", ""), 16,
		)
		if !ok {
			return nil, fmt.Errorf("failed to parse serial string")
		}

		cert := &x509.Certificate{
			SerialNumber: serialInt,
			Subject: pkix.Name{
				CommonName: "A TPM EKCert",
			},
			NotBefore: time.Now(),
			NotAfter:  time.Now().AddDate(0, 0, 1),
		}
		certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, ekKey, caKey)
		if err != nil {
			return nil, err
		}
		return certBytes, nil
	}

	return ekCertGenerator, caPEMBuf.Bytes(), nil

}

type tpmBehavior struct {
	incorrectAttestAK             bool
	incorrectAttestNonce          bool
	incorrectAttestPCR            bool
	incorrectAttestEvent          bool
	incorrectCredActivateSolution bool
	emptyEventLog                 bool

	// If specified, the simulator uses this to fetch an EKCert to submit
	// containing the EKPub, rather than submitting just the KEPub
	ekCertGenerator ekCertGenerator

	// If specified, during authn the simulator will sign the challenge nonce
	// with this signer and include the signature as SshSignature in the
	// challenge response.
	sshSigner crypto.Signer

	modifyEnrollDeviceInit func(r *devicepb.EnrollDeviceInit)
}

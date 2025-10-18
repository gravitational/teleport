package ec2join

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAWSCerts asserts that all certificates parse.
func TestAWSCerts(t *testing.T) {
	for _, certBytes := range awsRSA2048CertBytes {
		certPEM, _ := pem.Decode(certBytes)
		_, err := x509.ParseCertificate(certPEM.Bytes)
		require.NoError(t, err)
	}
}

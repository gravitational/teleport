package protocol

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func DecodePacketDump(dump string) ([]byte, error) {
	var sb strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(dump), "\n") {
		if len(line) < 58 {
			return nil, trace.BadParameter("invalid line length")
		}
		sb.WriteString(strings.ReplaceAll(line[10:58], " ", ""))
	}
	buff, err := hex.DecodeString(sb.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return buff, nil
}

func MustDecodePacketDump(t *testing.T, dump string) []byte {
	buff, err := DecodePacketDump(dump)
	require.NoError(t, err)
	return buff
}

func MustParseDumpToPacket(t *testing.T, dump string) Packet {
	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()
	conn := oracleConn{Conn: pr}
	defer conn.Close()
	go func() {
		_, err := pw.Write(MustDecodePacketDump(t, dump))
		assert.NoError(t, err)
	}()

	pck, err := conn.readPacket()
	assert.NoError(t, err)
	return pck
}

func MustCreateSelfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	keyPEM, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport Test"},
		CommonName:   "Teleport",
	}, []string{"localhost", "127.0.0.1"}, 10*365*24*time.Hour)
	require.NoError(t, err)
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(certPEM)
	require.True(t, ok)
	return certificate, pool
}

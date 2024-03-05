package quic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
	"golang.org/x/sync/errgroup"
)

func TestConn(t *testing.T) {
	require := require.New(t)

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(err)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "snakeoil"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(3650 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,

		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},

		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	require.NoError(err)

	l, err := quic.ListenAddr("127.0.0.1:", &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{der},
			PrivateKey:  privKey,
		}},
	}, nil)
	require.NoError(err)
	t.Cleanup(func() { _ = l.Close() })

	var q1, q2 quic.Connection
	eg, egCtx := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		var err error
		q1, err = l.Accept(egCtx)
		if err != nil {
			return err
		}
		t.Cleanup(func() {
			_ = q1.CloseWithError(quic.ApplicationErrorCode(0), "cleanup")
		})
		return nil
	})
	eg.Go(func() error {
		var err error
		q2, err = quic.DialAddr(egCtx, l.Addr().String(), &tls.Config{
			InsecureSkipVerify: true,
		}, nil)
		if err != nil {
			return err
		}
		t.Cleanup(func() {
			_ = q2.CloseWithError(quic.ApplicationErrorCode(0), "cleanup")
		})
		return nil
	})
	require.NoError(eg.Wait())

	var mu sync.Mutex
	nettest.TestConn(t, func() (c1 net.Conn, c2 net.Conn, stop func(), err error) {
		mu.Lock()
		defer mu.Unlock()
		var s1, s2 quic.Stream
		eg, egCtx := errgroup.WithContext(context.Background())
		eg.Go(func() error {
			var err error
			s1, err = q1.AcceptStream(egCtx)
			if err != nil {
				return err
			}
			_, err = s1.Read([]byte{0})
			if err != nil {
				s1.CancelRead(0)
				s1.CancelWrite(0)
				return err
			}
			return nil
		})
		eg.Go(func() error {
			var err error
			s2, err = q2.OpenStreamSync(egCtx)
			if err != nil {
				return err
			}
			_, err = s2.Write([]byte{0})
			if err != nil {
				s2.CancelRead(0)
				s2.CancelWrite(0)
				return err
			}
			return nil
		})
		if err := eg.Wait(); err != nil {
			return nil, nil, nil, err
		}
		c1 = newStreamConn(s1, nil, nil)
		c2 = newStreamConn(s2, nil, nil)

		return c1, c2, sync.OnceFunc(func() {
			c1.Close()
			c2.Close()
		}), nil
	})
}

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package relaytransport

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestSNIDispatch_NoDispatch(t *testing.T) {
	caCert := requireExampleServerCert(t)
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert.Leaf)

	var eg errgroup.Group
	defer eg.Wait()

	// net.Pipe is unbuffered and crypto/tls doesn't work well with unbuffered
	// connections (even though it should)
	sc, cc, err := uds.NewSocketpair(uds.SocketTypeStream)
	require.NoError(t, err)
	defer sc.Close()
	defer cc.Close()

	// from the point of view of a client, the SNI dispatcher does not exist
	eg.Go(func() error {
		defer cc.Close()
		tc := tls.Client(cc, &tls.Config{
			ServerName: "foo.example.com",
			RootCAs:    caPool,
		})
		if err := tc.Handshake(); err != nil {
			return err
		}
		if _, err := tc.Write([]byte("a")); err != nil {
			return err
		}
		if _, err := io.ReadFull(tc, make([]byte, 1)); err != nil {
			return err
		}
		return nil
	})

	// a connection going through the dispatcher with the wrong SNI will not be
	// dispatched, and it will be passed to the inner TransportCredentials
	var transportCredentialsCalled, dispatchFuncCalled int
	var dispatchErr error
	returnedConn, _, err := (&SNIDispatchTransportCredentials{
		DispatchFunc: func(serverName string, transcript *bytes.Buffer, rawConn net.Conn) (dispatched bool) {
			dispatchFuncCalled++
			if serverName != "foo.example.com" {
				dispatchErr = fmt.Errorf("expected foo.example.com, got SNI %+q", serverName)
			}
			return false
		},
		TransportCredentials: transportCredentialsFunc(func(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
			transportCredentialsCalled++
			return rawConn, nil, nil
		}),
	}).ServerHandshake(sc)
	require.NoError(t, err)
	require.NoError(t, dispatchErr)
	require.Equal(t, 1, dispatchFuncCalled)
	require.Equal(t, 1, transportCredentialsCalled)

	// prove that we can use the non-dispatched conn as is
	tc := tls.Server(returnedConn, &tls.Config{
		Certificates: []tls.Certificate{caCert},
	})
	require.NoError(t, tc.Handshake())

	_, err = tc.Write([]byte("a"))
	require.NoError(t, err)
	_, err = io.ReadFull(tc, make([]byte, 1))
	require.NoError(t, err)

	// the client is happy
	require.NoError(t, eg.Wait())
}

func TestSNIDispatch_YesDispatch(t *testing.T) {
	caCert := requireExampleServerCert(t)
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert.Leaf)

	var eg errgroup.Group
	defer eg.Wait()

	// net.Pipe is unbuffered and crypto/tls doesn't work well with unbuffered
	// connections (even though it should)
	sc, cc, err := uds.NewSocketpair(uds.SocketTypeStream)
	require.NoError(t, err)
	defer sc.Close()
	defer cc.Close()

	// from the point of view of a client, the SNI dispatcher does not exist
	eg.Go(func() error {
		defer cc.Close()
		tc := tls.Client(cc, &tls.Config{
			ServerName: "bar.example.com",
			RootCAs:    caPool,
		})
		if err := tc.Handshake(); err != nil {
			return err
		}
		if _, err := tc.Write([]byte("a")); err != nil {
			return err
		}
		if _, err := io.ReadFull(tc, make([]byte, 1)); err != nil {
			return err
		}
		return nil
	})

	// a connection going through the dispatcher with the correct SNI will be
	// dispatched and the inner TransportCredentials doesn't see it
	var transportCredentialsCalled, dispatchFuncCalled int
	var dispatchErr error
	var dispatchedTranscript *bytes.Buffer
	var dispatchedConn net.Conn
	_, _, err = (&SNIDispatchTransportCredentials{
		DispatchFunc: func(serverName string, transcript *bytes.Buffer, rawConn net.Conn) (dispatched bool) {
			dispatchFuncCalled++
			if serverName != "bar.example.com" {
				dispatchErr = fmt.Errorf("expected bar.example.com, got SNI %+q", serverName)
				return false
			}
			dispatchedTranscript = transcript
			dispatchedConn = rawConn
			return true
		},
		TransportCredentials: transportCredentialsFunc(func(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
			transportCredentialsCalled++
			return rawConn, nil, nil
		}),
	}).ServerHandshake(sc)
	require.ErrorIs(t, err, credentials.ErrConnDispatched)
	require.NoError(t, dispatchErr)
	require.Equal(t, 1, dispatchFuncCalled)
	require.Equal(t, 0, transportCredentialsCalled)

	// the DispatchFunc gets the actual connection object plus a transcript
	require.Same(t, sc, dispatchedConn)

	// a TLS ClientHello is in there
	require.NotNil(t, dispatchedTranscript)
	require.NotZero(t, dispatchedTranscript.Len())

	type nestedConn struct {
		net.Conn
	}
	type bypassReader struct {
		io.Reader
		nestedConn
	}
	// methods in the io.Reader will take priority over nestedConn
	scWithTranscript := bypassReader{
		Reader:     io.MultiReader(dispatchedTranscript, dispatchedConn),
		nestedConn: nestedConn{dispatchedConn},
	}

	// prove that we can use the dispatched conn if it's preceded by the
	// returned transcript
	tc := tls.Server(scWithTranscript, &tls.Config{
		Certificates: []tls.Certificate{caCert},
	})
	require.NoError(t, tc.Handshake())

	_, err = tc.Write([]byte("a"))
	require.NoError(t, err)
	_, err = io.ReadFull(tc, make([]byte, 1))
	require.NoError(t, err)

	// the client is happy
	require.NoError(t, eg.Wait())
}

type transportCredentialsFunc func(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error)

var _ credentials.TransportCredentials = transportCredentialsFunc(nil)

// ServerHandshake implements [credentials.TransportCredentials].
func (fn transportCredentialsFunc) ServerHandshake(c net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return fn(c)
}

// ClientHandshake implements [credentials.TransportCredentials].
func (transportCredentialsFunc) ClientHandshake(context.Context, string, net.Conn) (net.Conn, credentials.AuthInfo, error) {
	panic("unimplemented")
}

// Clone implements [credentials.TransportCredentials].
func (transportCredentialsFunc) Clone() credentials.TransportCredentials {
	panic("unimplemented")
}

// Info implements [credentials.TransportCredentials].
func (transportCredentialsFunc) Info() credentials.ProtocolInfo {
	panic("unimplemented")
}

// OverrideServerName implements [credentials.TransportCredentials].
func (transportCredentialsFunc) OverrideServerName(string) error {
	panic("unimplemented")
}

func requireExampleServerCert(t *testing.T) tls.Certificate {
	t.Helper()

	_, key, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	cert := &x509.Certificate{
		NotBefore:   time.Now().Add(-24 * time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		BasicConstraintsValid: true,
		IsCA:                  true,

		DNSNames: []string{"*.example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, key.Public(), key)
	require.NoError(t, err)

	cert, err = x509.ParseCertificate(der)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        cert,
	}
}

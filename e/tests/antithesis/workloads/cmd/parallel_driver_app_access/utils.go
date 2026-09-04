package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
	"github.com/gravitational/teleport/lib/cryptosuites"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

func appSessionID(cert tls.Certificate) string {
	if len(cert.Certificate) == 0 {
		return ""
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return ""
	}
	identity, err := tlsca.FromSubject(leaf.Subject, leaf.NotAfter)
	if err != nil {
		return ""
	}
	return identity.RouteToApp.SessionID
}

func fetchAppBody(ctx context.Context, cert tls.Certificate, publicAddr, marker string) (string, int, error) {
	appHost := strings.Split(publicAddr, ":")[0]
	proxyAddr := testenv.ProxyAddr()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   appHost,
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, proxyAddr)
		},
	}
	defer tr.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+publicAddr+"/", nil)
	if err != nil {
		return "", 0, trace.Wrap(err, "building request")
	}
	req.Header.Set("X-Antithesis-App-Marker", marker)

	clt := &http.Client{Transport: tr}
	resp, err := clt.Do(req)
	if err != nil {
		return "", 0, trace.ConnectionProblem(err, "requesting app")
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", resp.StatusCode, trace.ConnectionProblem(err, "reading app response body")
	}

	return strings.TrimSpace(string(bodyBytes)), resp.StatusCode, nil
}

func fetchTCPAppBody(ctx context.Context, cert tls.Certificate, publicAddr, marker string) (string, int, error) {
	// Note we do not set a deadline on this context, let the fuzzer hang if it hangs. Right now there are
	// no properties to assert timeouts and this would only add more noise to the simulation.
	tlsConn, err := apiclient.DialALPN(ctx, testenv.ProxyAddr(), apiclient.ALPNDialerConfig{
		TLSConfig: &tls.Config{
			NextProtos:   alpncommon.ProtocolToStringsWithPing(alpncommon.ProtocolTCP),
			Certificates: []tls.Certificate{cert},
		},
	})
	if err != nil {
		return "", 0, trace.ConnectionProblem(err, "dialing TCP app")
	}

	var conn net.Conn = tlsConn
	if alpncommon.IsPingProtocol(alpncommon.Protocol(tlsConn.ConnectionState().NegotiatedProtocol)) {
		conn = pingconn.NewTLS(tlsConn)
	}
	defer conn.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+publicAddr+"/", nil)
	if err != nil {
		return "", 0, trace.Wrap(err, "building TCP app request")
	}

	// This allows nginx not to keep the connection open which should end the session and generate an event.
	req.Header.Set("Connection", "close")

	req.Header.Set("X-Antithesis-App-Marker", marker)

	if err := req.Write(conn); err != nil {
		return "", 0, trace.ConnectionProblem(err, "writing TCP app request")
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return "", 0, trace.ConnectionProblem(err, "reading TCP app response")
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", resp.StatusCode, trace.ConnectionProblem(err, "reading TCP app response body")
	}

	return strings.TrimSpace(string(bodyBytes)), resp.StatusCode, nil
}

func mintAppCert(ctx context.Context, clt *apiclient.Client, username string, app AppTarget) (tls.Certificate, error) {
	priv, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "generating private key")
	}
	keyPEM, err := keys.MarshalPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "marshaling private key")
	}
	pubPEM, err := keys.MarshalPublicKey(priv.Public())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "marshaling public key")
	}

	certs, err := clt.GenerateUserCerts(ctx, proto.UserCertsRequest{
		TLSPublicKey:  pubPEM,
		Username:      username,
		Usage:         proto.UserCertsRequest_App,
		RequesterName: proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
		Expires:       time.Now().Add(time.Hour),
		RouteToApp: proto.RouteToApp{
			Name:        app.Name,
			PublicAddr:  app.PublicAddr,
			ClusterName: testenv.DefaultClusterName,
			URI:         app.URI,
		},
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	cert, err := keys.X509KeyPair(certs.TLS, keyPEM)
	return cert, trace.Wrap(err)
}

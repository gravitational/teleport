package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{DisableHTMLEscape: true})

	ctx := context.Background()

	prof, err := profile.FromDir("", "")
	if err != nil {
		panic(err)
	}

	clt, err := apiclient.New(ctx, apiclient.Config{
		Addrs:       []string{prof.WebProxyAddr},
		Credentials: []apiclient.Credentials{apiclient.LoadProfile("", "")},

		ALPNSNIAuthDialClusterName: prof.SiteName,

		Context: ctx,
	})
	if err != nil {
		panic(err)
	}
	defer clt.Close()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	pubKey := privKey.Public()
	pubKeyPKIX, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		panic(err)
	}

	resp, err := clt.GenerateUserAppHostCerts(ctx, &proto.GenerateUserAppHostCertsRequest{
		PublicKeyPkix: pubKeyPKIX,
	})
	if err != nil {
		panic(err)
	}

	privKeyDer, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		panic(err)
	}

	privKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyDer,
	})

	id, err := auth.ReadIdentityFromKeyPair(privKeyPem, resp.Certs)
	if err != nil {
		panic(err)
	}

	clt2tls, err := id.TLSConfig(nil)
	if err != nil {
		panic(err)
	}
	clt2tls.ClientAuth = tls.RequireAndVerifyClientCert

	userCA, err := clt.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: prof.SiteName,
	}, false)
	if err != nil {
		panic(err)
	}

	clt2tls.ClientCAs = x509.NewCertPool()
	for _, k := range userCA.GetTrustedTLSKeyPairs() {
		if !clt2tls.ClientCAs.AppendCertsFromPEM(k.Cert) {
			panic(false)
		}
	}

	fileServer, err := newFileServer()
	if err != nil {
		panic(err)
	}

	fileServer.mu.Lock()
	fileServer.shares = map[string]FileServerShare{
		"pwd": {
			Path: ".",
			AllowedRolesList: []string{
				"access",
			},
		},
		"root": {
			Path: "/",
			AllowedRolesList: []string{
				"editor",
			},
		},
	}
	fileServer.mu.Unlock()

	fileServerListener := make(listenerChan)
	fileServer.srv.TLSConfig = clt2tls
	go fileServer.srv.ServeTLS(fileServerListener, "", "")

	clt2, err := auth.NewClient(apiclient.Config{
		Addrs: []string{prof.WebProxyAddr},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(clt2tls),
		},

		ALPNSNIAuthDialClusterName: prof.SiteName,

		Context: ctx,
	})
	if err != nil {
		panic(err)
	}
	defer clt2.Close()

	resolverMode := types.ProxyListenerMode_Separate
	if prof.TLSRoutingEnabled {
		resolverMode = types.ProxyListenerMode_Multiplex
	}
	agentPool, err := reversetunnel.NewAgentPool(ctx, reversetunnel.AgentPoolConfig{
		Client:       clt2,
		AccessPoint:  clt2,
		HostSigner:   id.KeySigner,
		HostUUID:     id.ID.HostUUID,
		LocalCluster: id.ClusterName,

		Server: fileServerListener,

		Component: "userapp",

		Resolver: reversetunnelclient.StaticResolver(prof.WebProxyAddr, resolverMode),
		Cluster:  id.ClusterName,
	})
	if err != nil {
		panic(err)
	}
	agentPool.Start()
	defer agentPool.Stop()

	clt2.UpsertApplicationServer(ctx, &types.AppServerV3{
		Spec: types.AppServerSpecV3{
			ProxyIDs: agentPool.GetConnectedProxyGetter().GetProxyIDs(),
			App:      &types.AppV3{},
		},
	})

	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for range tick.C {
		_, err = clt2.UpsertApplicationServer(ctx, &types.AppServerV3{
			Spec: types.AppServerSpecV3{
				ProxyIDs: agentPool.GetConnectedProxyGetter().GetProxyIDs(),
				App:      &types.AppV3{},
			},
		})
		if err != nil {
			panic(err)
		}
		logrus.Info("sent heartbeat")
	}
}

type listenerChan chan net.Conn

var (
	_ net.Listener                = listenerChan(nil)
	_ reversetunnel.ServerHandler = listenerChan(nil)
)

type waitableConn struct {
	net.Conn
	ch   chan struct{}
	once sync.Once
}

func (w *waitableConn) Chan() <-chan struct{} {
	return w.ch
}

func (w *waitableConn) Close() error {
	defer w.once.Do(func() { close(w.ch) })
	return w.Conn.Close()
}

// HandleConnection implements reversetunnel.ServerHandler.
func (l listenerChan) HandleConnection(conn net.Conn) {
	ch := make(chan struct{})
	l <- &waitableConn{
		Conn: conn,
		ch:   ch,
	}
	<-ch
}

// Accept implements net.Listener.
func (l listenerChan) Accept() (net.Conn, error) {
	return <-l, nil
}

// Addr implements net.Listener.
func (listenerChan) Addr() net.Addr {
	return nil
}

// Close implements net.Listener.
func (listenerChan) Close() error {
	return nil
}

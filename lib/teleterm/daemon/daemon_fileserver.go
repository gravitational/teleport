package daemon

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	user2 "os/user"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/utils"
)

func (s *Service) StartFileServers() error {
	clusters, err := s.cfg.Storage.ReadAll()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, c := range clusters {
		if !c.Connected() {
			continue
		}

		err := s.StartFileServer(context.TODO(), c.URI.String())

		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *Service) StopFileServer(clusterURI string) {
	s.fileServersMu.Lock()
	defer s.fileServersMu.Unlock()
	fs, ok := s.fileServers[clusterURI]
	if !ok {
		return
	}

	fs.Stop()
	delete(s.fileServers, clusterURI)
}

func (s *Service) StartFileServer(ctx context.Context, clusterURI string) error {
	_, tc, err := s.ResolveCluster(clusterURI)
	if err != nil {
		return trace.Wrap(err)
	}
	keys, err := jwks(ctx, tc)

	s.fileServersMu.Lock()
	defer s.fileServersMu.Unlock()
	return s.startFileServerLocked(clusterURI, tc, keys)
}

func (s *Service) startFileServerLocked(clusterURI string, tc *client.TeleportClient, keys []*jwt.Key) error {
	if fs, ok := s.fileServers[clusterURI]; ok {
		fs.Stop()
	}

	if s.fileServers == nil {
		s.fileServers = make(map[string]*fileServer)
	}
	fs, err := newFileServer(tc, keys)
	if err != nil {
		return trace.Wrap(err)
	}

	s.fileServers[clusterURI] = fs
	return nil
}

func jwks(ctx context.Context, tc *client.TeleportClient) ([]*jwt.Key, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := proxyClient.ConnectToCluster(ctx, proxyClient.ClusterName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Fetch the JWT public keys only.
	ca, err := cluster.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: proxyClient.ClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pairs := ca.GetTrustedJWTKeyPairs()

	// Create response and allocate space for the keys.
	var out []*jwt.Key

	// Loop over and all add public keys in JWK format.
	for _, pair := range pairs {
		publicKey, err := utils.ParsePublicKey(pair.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg := &jwt.Config{
			Algorithm:   defaults.ApplicationTokenAlgorithm,
			ClusterName: ca.GetClusterName(),
			PublicKey:   publicKey,
		}
		key, err := jwt.New(cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, key)
	}
	return out, nil
}

func newFileServer(tc *client.TeleportClient, keys []*jwt.Key) (*fileServer, error) {
	u, err := user2.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fs := new(fileServer)
	fs.shares = map[string]fileServerShare{
		"temp": {
			path:             "/tmp",
			AllowAnyone:      false,
			AllowedUsersList: nil,
			AllowedRolesList: []string{"access"},
		},
		"usr": {
			path:             "/usr/bin",
			AllowAnyone:      true,
			AllowedUsersList: nil,
			AllowedRolesList: nil,
		},
		"home": {
			path:             u.HomeDir,
			AllowAnyone:      false,
			AllowedUsersList: []string{tc.Username},
			AllowedRolesList: nil,
		},
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fs.port = lis.Addr().(*net.TCPAddr).Port

	mux := httprouter.New()
	mux.GET("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// TODO(not espadolini): prettier landing page
		_, _ = w.Write([]byte("welcome to Teleport Connect file sharing"))
	})
	mux.GET("/:shareName", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		http.Redirect(w, r, r.URL.EscapedPath()+"/", 302)
	})
	mux.GET("/:shareName/*filePath", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		shareName := p.ByName("shareName")
		filePath := p.ByName("filePath")

		token := r.Header.Get(teleport.AppJWTHeader)
		if token == "" {
			log.Warningf("missing header %q", teleport.AppJWTHeader)
			http.NotFound(w, r)
			return
		}

		var claims *jwt.Claims
		var err error
		for _, key := range keys {
			claims, err = key.VerifyUnknownUser(jwt.VerifyParams{
				RawToken: token,
				Audience: tc.SiteName,
				URI:      fmt.Sprintf("https://127.0.0.1:%v/", fs.port),
			})
			if err != nil {
				log.WithError(err).Warnf("fail: %v", r.URL.String())
			} else {
				log.Infof("verified token against key %v", key)
			}
		}

		fs.mu.Lock()
		share, found := fs.shares[shareName]
		var path string
		if !found || !share.CanAccess(claims) {
			http.NotFound(w, r)
			return
		}
		fs.mu.Unlock()

		if path == "" {
			http.NotFound(w, r)
			return
		}

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL

		r2.URL.Path = filePath
		r2.URL.RawPath = ""

		// TODO(not espadolini): make the listing prettier
		http.FileServer(http.Dir(share.path)).ServeHTTP(w, r2)
	})

	fs.srv.Handler = mux
	fs.srv = http.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{generateSelfSignedCert()},
			MinVersion:   tls.VersionTLS13,
		},

		// ReadTimeout:       0,
		// ReadHeaderTimeout: 0,
		// WriteTimeout:      0,
		// IdleTimeout:       0,
		// MaxHeaderBytes:    0,
	}

	go func() {
		defer lis.Close()
		fs.srv.ServeTLS(lis, "", "")
	}()

	return fs, nil
}

type fileServer struct {
	srv  http.Server
	port int

	mu     sync.Mutex
	shares map[string]fileServerShare
}

type fileServerShare struct {
	path string

	AllowAnyone      bool
	AllowedUsersList []string
	AllowedRolesList []string
}

func (fss fileServerShare) CanAccess(claims *jwt.Claims) bool {
	if fss.AllowAnyone {
		return true
	}

	if slices.Contains(fss.AllowedUsersList, claims.Username) {
		return true
	}

	for _, role := range claims.Roles {
		if slices.Contains(fss.AllowedRolesList, role) {
			return true
		}
	}

	return false
}

func (f *fileServer) Stop() {
	f.srv.Close()
}

func generateSelfSignedCert() tls.Certificate {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(trace.Wrap(err))
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		panic(trace.Wrap(err))
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Issuer:       pkix.Name{CommonName: "connect file share"},
		Subject:      pkix.Name{CommonName: "connect file share"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		panic(trace.Wrap(err))
	}

	return tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privKey,
	}
}

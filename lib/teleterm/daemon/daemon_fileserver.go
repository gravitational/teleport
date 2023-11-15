// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"sync"
	"text/template"
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
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	keys, err := jwks(ctx, proxyClient)
	if err != nil {
		return trace.Wrap(err)
	}

	s.fileServersMu.Lock()
	defer s.fileServersMu.Unlock()

	return s.startFileServerLocked(clusterURI, keys)
}

func (s *Service) startFileServerLocked(clusterURI string, keys []*jwt.Key) error {
	if fs, ok := s.fileServers[clusterURI]; ok {
		fs.Stop()
	}

	if s.fileServers == nil {
		s.fileServers = make(map[string]*fileServer)
	}
	fs, err := newFileServer(keys)
	if err != nil {
		return trace.Wrap(err)
	}

	s.fileServers[clusterURI] = fs
	return nil
}

func jwks(ctx context.Context, proxyClient *client.ProxyClient) ([]*jwt.Key, error) {
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

func (s *Service) GetFileServerConfig(ctx context.Context, clusterURI string) (map[string]FileServerShare, error) {
	s.fileServersMu.Lock()
	defer s.fileServersMu.Unlock()

	fileServer, ok := s.fileServers[clusterURI]
	if !ok {
		return nil, trace.NotFound("cluster URI not found")
	}

	fileServer.mu.Lock()
	defer fileServer.mu.Unlock()

	r := make(map[string]FileServerShare, len(fileServer.shares))
	for k, v := range fileServer.shares {
		r[k] = FileServerShare{
			Path:             v.Path,
			AllowAnyone:      v.AllowAnyone,
			AllowedUsersList: slices.Clone(v.AllowedUsersList),
			AllowedRolesList: slices.Clone(v.AllowedRolesList),
		}
	}

	return r, nil
}

func (s *Service) SetFileServerConfig(ctx context.Context, clusterURI string, shares map[string]FileServerShare) error {
	s.fileServersMu.Lock()
	defer s.fileServersMu.Unlock()

	fileServer, ok := s.fileServers[clusterURI]
	if !ok {
		return trace.NotFound("cluster URI not found")
	}

	fileServer.mu.Lock()
	defer fileServer.mu.Unlock()

	fileServer.shares = shares
	return nil
}

func getClaimsFromRequest(port int, keys []*jwt.Key, r *http.Request) (*jwt.Claims, error) {
	token := r.Header.Get(teleport.AppJWTHeader)
	if token == "" {
		return nil, trace.BadParameter("missing header %q", teleport.AppJWTHeader)
	}

	for _, key := range keys {
		claims, err := key.VerifyUnknownUser(jwt.VerifyParams{
			RawToken: token,
			URI:      fmt.Sprintf("https://127.0.0.1:%v", port),
		})
		if err != nil {
			log.WithError(err).Infof("token validation failed: %v", r.URL.String())
			continue
		}

		log.Infof("verified token for username %q", claims.Username)
		return claims, err
	}

	return nil, trace.BadParameter("token failed to validate with any of keys")
}

func newFileServer(keys []*jwt.Key) (*fileServer, error) {
	fs := new(fileServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fs.port = lis.Addr().(*net.TCPAddr).Port

	mux := httprouter.New()
	mux.GET("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		claims, err := getClaimsFromRequest(fs.port, keys, r)
		if err != nil {
			log.WithError(err).Infof("validation failed")
			http.Error(w, "Authentication failed.", http.StatusForbidden)
			return
		}

		var accessible []string
		fs.mu.Lock()

		for shareName, share := range fs.shares {
			if share.CanAccess(claims) {
				accessible = append(accessible, shareName)
			}
		}
		fs.mu.Unlock()

		tmpl := `
		<!DOCTYPE html>
		<html lang="en">
		<body>
			<h2>Welcome to Teleport Connect file sharing!</h2>
			<ul>
				{{range .}}
					<li><a href="{{.}}">{{.}}</a></li>
				{{end}}
			</ul>
		</body>
		</html>
`

		t, err := template.New("index").Parse(tmpl)
		if err != nil {
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}

		err = t.Execute(w, accessible)
		if err != nil {
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
	})
	mux.GET("/:shareName", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		http.Redirect(w, r, r.URL.EscapedPath()+"/", 302)
	})
	mux.GET("/:shareName/*filePath", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		shareName := p.ByName("shareName")
		filePath := p.ByName("filePath")

		claims, err := getClaimsFromRequest(fs.port, keys, r)
		if err != nil {
			log.WithError(err).Infof("validation failed")
			http.Error(w, "Authentication failed.", http.StatusForbidden)
			return
		}

		var path string
		fs.mu.Lock()
		if share, ok := fs.shares[shareName]; ok {
			if share.CanAccess(claims) {
				path = share.Path
			} else {
				log.Infof("user %q attempted to access share %q, no access", claims.Username, shareName)
			}
		} else {
			log.Infof("user %q attempted to access share %q, does not exist", claims.Username, shareName)
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

		log.Infof("all good, serving path %q from share %q to user %q", path, shareName, claims.Username)
		// TODO(not espadolini): make the listing prettier
		http.FileServer(http.Dir(path)).ServeHTTP(w, r2)
	})

	fs.srv = http.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{generateSelfSignedCert()},
			MinVersion:   tls.VersionTLS13,
		},
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
	shares map[string]FileServerShare
}

type FileServerShare struct {
	Path string

	AllowAnyone      bool
	AllowedUsersList []string
	AllowedRolesList []string
}

func (fss FileServerShare) CanAccess(claims *jwt.Claims) bool {
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

package daemon

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (s *Service) StartFileServers() error {
	s.fileServersMu.Lock()
	defer s.fileServersMu.Unlock()

	clusters, err := s.cfg.Storage.ReadAll()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, c := range clusters {
		if !c.Connected() {
			continue
		}

		s.startFileServerLocked(c.URI.String())
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

func (s *Service) StartFileServer(clusterURI string) {
	s.fileServersMu.Lock()
	defer s.fileServersMu.Unlock()
	s.startFileServerLocked(clusterURI)
}

func (s *Service) startFileServerLocked(clusterURI string) {
	if fs, ok := s.fileServers[clusterURI]; ok {
		fs.Stop()
	}

	if s.fileServers == nil {
		s.fileServers = make(map[string]*fileServer)
	}
	s.fileServers[clusterURI] = newFileServer()
}

func newFileServer() *fileServer {
	fs := new(fileServer)

	mux := httprouter.New()
	mux.GET("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// TODO(not espadolini): prettier landing page
		w.Write([]byte("welcome to Teleport Connect file sharing"))
	})
	mux.GET("/:shareName", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		http.Redirect(w, r, r.URL.EscapedPath()+"/", 302)
	})
	mux.GET("/:shareName/*filePath", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		shareName := p[0].Value
		filePath := p[1].Value

		fs.mu.Lock()
		share, found := fs.shares[shareName]
		var path string
		if found {
			path = share.path
			// TODO: check permissions
			// if permissions fail, set path = false
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

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(trace.Wrap(err))
	}

	fs.port = lis.Addr().(*net.TCPAddr).Port

	go func() {
		defer lis.Close()
		fs.srv.ServeTLS(lis, "", "")
	}()

	return fs
}

type fileServer struct {
	srv  http.Server
	port int

	mu     sync.Mutex
	shares map[string]fileServerShare
}

type fileServerShare struct {
	path string
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

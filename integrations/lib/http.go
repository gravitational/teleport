/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package lib

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// TLSConfig stores TLS configuration for a http service
type TLSConfig struct {
	VerifyClientCertificate bool `toml:"verify_client_cert"`

	VerifyClientCertificateFunc func(chains [][]*x509.Certificate) error
}

// HTTPConfig stores configuration of an HTTP service
// including its public address, listen host and port,
// TLS certificate and key path, and extra TLS configuration
// options, represented as TLSConfig.
type HTTPConfig struct {
	ListenAddr string              `toml:"listen_addr"`
	PublicAddr string              `toml:"public_addr"`
	KeyFile    string              `toml:"https_key_file"`
	CertFile   string              `toml:"https_cert_file"`
	BasicAuth  HTTPBasicAuthConfig `toml:"basic_auth"`
	TLS        TLSConfig           `toml:"tls"`

	Insecure bool
}

// HTTPBasicAuthConfig stores configuration for
// HTTP Basic Authentication
type HTTPBasicAuthConfig struct {
	Username string `toml:"user"`
	Password string `toml:"password"`
}

// HTTP is a tiny wrapper around standard net/http.
// It starts either insecure server or secure one with TLS, depending on the settings.
// It also adds a context to its handlers and the server itself has context to.
// So you are guaranteed that server will be closed when the context is canceled.
type HTTP struct {
	HTTPConfig
	mu      sync.Mutex
	addr    net.Addr
	baseURL *url.URL
	*httprouter.Router
	server http.Server
}

// HTTPBasicAuth wraps a http.Handler with HTTP Basic Auth check.
type HTTPBasicAuth struct {
	HTTPBasicAuthConfig
	handler http.Handler
}

type httpListenChanKey struct{}

func (conf *HTTPConfig) defaultScheme() (scheme string) {
	if conf.Insecure {
		scheme = "http"
	} else {
		scheme = "https"
	}
	return
}

// BaseURL builds a base url depending on "public_addr" parameter.
func (conf *HTTPConfig) BaseURL() (*url.URL, error) {
	if conf.PublicAddr == "" {
		return &url.URL{Scheme: conf.defaultScheme()}, nil
	}
	url, err := url.Parse(conf.PublicAddr)
	if err != nil {
		return nil, err
	}

	scheme := url.Scheme
	if scheme == "" {
		scheme = conf.defaultScheme()
		return url.Parse(fmt.Sprintf("%s://%s", scheme, conf.PublicAddr))
	}

	if scheme != "http" && scheme != "https" {
		return nil, trace.BadParameter("wrong scheme in public_addr parameter: %q", scheme)
	}

	return url, nil
}

// Check validates the http server configuration.
func (conf *HTTPConfig) Check() error {
	baseURL, err := conf.BaseURL()
	if err != nil {
		return trace.Wrap(err)
	}
	if conf.KeyFile != "" && conf.CertFile == "" {
		return trace.BadParameter("https_cert_file is required when https_key_file is specified")
	}
	if conf.CertFile != "" && conf.KeyFile == "" {
		return trace.BadParameter("https_key_file is required when https_cert_file is specified")
	}
	if conf.BasicAuth.Password != "" && conf.BasicAuth.Username == "" {
		return trace.BadParameter("basic_auth.user is required when basic_auth.password is specified")
	}
	if conf.BasicAuth.Username != "" && baseURL != nil && baseURL.User != nil {
		return trace.BadParameter("passing credentials both in basic_auth section and public_addr parameter is not supported")
	}
	return nil
}

// ServeHTTP processes one http request.
func (auth *HTTPBasicAuth) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()

	if ok && username == auth.Username && password == auth.Password {
		auth.handler.ServeHTTP(rw, r)
	} else {
		rw.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
		http.Error(rw, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}

// NewHTTP creates a new HTTP wrapper
func NewHTTP(config HTTPConfig) (*HTTP, error) {
	baseURL, err := config.BaseURL()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	router := httprouter.New()

	if userInfo := baseURL.User; userInfo != nil {
		password, _ := userInfo.Password()
		config.BasicAuth = HTTPBasicAuthConfig{Username: userInfo.Username(), Password: password}
	}

	var handler http.Handler
	handler = router
	if config.BasicAuth.Username != "" {
		handler = &HTTPBasicAuth{config.BasicAuth, handler}
	}

	var tlsConfig *tls.Config
	if !config.Insecure {
		tlsConfig = &tls.Config{}
		if config.TLS.VerifyClientCertificate {
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			if verify := config.TLS.VerifyClientCertificateFunc; verify != nil {
				tlsConfig.VerifyPeerCertificate = func(_ [][]byte, chains [][]*x509.Certificate) error {
					if err := verify(chains); err != nil {
						slog.ErrorContext(context.Background(), "HTTPS client certificate verification failed", "error", err)
						return err
					}
					return nil
				}
			}
		} else {
			tlsConfig.ClientAuth = tls.NoClientCert
		}
	}

	return &HTTP{
		HTTPConfig: config,
		Router:     router,
		baseURL:    baseURL,
		server:     http.Server{Handler: handler, TLSConfig: tlsConfig},
	}, nil
}

// BuildURLPath returns a URI with args represented as query params
// If any supplied argument is not a string, BuildURLPath will use
// fmt.Sprintf(value) to stringify it.
func BuildURLPath(args ...interface{}) string {
	var pathArgs []string
	for _, a := range args {
		var str string
		switch v := a.(type) {
		case string:
			str = v
		default:
			str = fmt.Sprint(v)
		}
		pathArgs = append(pathArgs, url.PathEscape(str))
	}
	return path.Join(pathArgs...)
}

// ListenAndServe runs a http(s) server on a provided port.
func (h *HTTP) ListenAndServe(ctx context.Context) error {
	defer slog.DebugContext(ctx, "HTTP server terminated")
	var err error

	h.server.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}
	go func() {
		<-ctx.Done()
		h.server.Close()
	}()

	listen := h.ListenAddr
	if listen == "" {
		if h.Insecure {
			listen = ":http"
		} else {
			listen = ":https"
		}
	}

	listenCh, _ := ctx.Value(httpListenChanKey{}).(chan<- net.Addr)
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		if listenCh != nil {
			listenCh <- nil
		}
		return trace.Wrap(err)
	}
	addr := listener.Addr()

	h.mu.Lock()
	h.addr = addr
	h.mu.Unlock()

	if listenCh != nil {
		listenCh <- addr
	}

	if h.Insecure {
		slog.DebugContext(ctx, "Starting insecure HTTP server", "listen_addr", logutils.StringerAttr(addr))
		err = h.server.Serve(listener)
	} else {
		slog.DebugContext(ctx, "Starting secure HTTPS server", "listen_addr", logutils.StringerAttr(addr))
		err = h.server.ServeTLS(listener, h.CertFile, h.KeyFile)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return trace.Wrap(err)
}

// Shutdown stops the server gracefully.
func (h *HTTP) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

// ShutdownWithTimeout stops the server gracefully.
func (h *HTTP) ShutdownWithTimeout(ctx context.Context, duration time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	return h.Shutdown(ctx)
}

// ServiceJob creates a service job for the HTTP service,
// wraps it with a termination handler so it shuts down and
// logs when it quits.
func (h *HTTP) ServiceJob() ServiceJob {
	return NewServiceJob(func(ctx context.Context) error {
		MustGetProcess(ctx).OnTerminate(func(ctx context.Context) error {
			if err := h.ShutdownWithTimeout(ctx, time.Second*5); err != nil {
				slog.ErrorContext(ctx, "HTTP server graceful shutdown failed")
				return err
			}
			return nil
		})
		listenChan := make(chan net.Addr)
		var outChan chan<- net.Addr = listenChan
		ctx = context.WithValue(ctx, httpListenChanKey{}, outChan)
		go func() {
			addr := <-listenChan
			close(listenChan)
			MustGetServiceJob(ctx).SetReady(addr != nil)
		}()
		return h.ListenAndServe(ctx)
	})
}

// BaseURL returns an url on which the server is accessible externally.
func (h *HTTP) BaseURL() *url.URL {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := *h.baseURL
	if url.Host == "" && h.addr != nil {
		url.Host = h.addr.String()
	}
	return &url
}

// NewURL builds an external url for a specific path and query parameters.
func (h *HTTP) NewURL(subpath string, values url.Values) *url.URL {
	url := h.BaseURL()
	url.Path = path.Join(url.Path, subpath)

	if values != nil {
		url.RawQuery = values.Encode()
	}

	return url
}

// EnsureCert checks cert and key files consistency.
func (h *HTTP) EnsureCert(defaultPath string) error {
	if h.Insecure {
		return nil
	}

	if h.CertFile != "" && h.KeyFile == "" {
		return trace.Errorf("you should specify https_key_file parameter")
	}

	if h.CertFile == "" && h.KeyFile != "" {
		return trace.Errorf("you should specify https_cert_file parameter")
	}

	_, err := tls.LoadX509KeyPair(h.CertFile, h.KeyFile)
	return trace.Wrap(err)
}

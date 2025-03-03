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

package alpnproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// certReissueClientWait is a duration for which Kube HTTP middleware will wait for cert reissuing until
// returning error to the client, indicating that local kube proxy requires user input. It should be short to
// bring user's attention to the local proxy sooner. It doesn't abort cert reissuing itself.
const certReissueClientWait = time.Second * 3

// certReissueClientWaitHeadless is used when proxy works in headless mode - since user works in reexeced shell,
// we give them longer time to perform the headless login flow.
const certReissueClientWaitHeadless = defaults.HeadlessLoginTimeout

// KubeClientCerts is a map of Kubernetes client certs.
type KubeClientCerts map[string]tls.Certificate

// Add adds a tls.Certificate for a kube cluster.
func (c KubeClientCerts) Add(teleportCluster, kubeCluster string, cert tls.Certificate) {
	c[common.KubeLocalProxySNI(teleportCluster, kubeCluster)] = cert
}

// KubeCertReissuer reissues a client certificate for a Kubernetes cluster.
type KubeCertReissuer = func(ctx context.Context, teleportCluster, kubeCluster string) (tls.Certificate, error)

// KubeMiddleware is a LocalProxyHTTPMiddleware for handling Kubernetes
// requests.
type KubeMiddleware struct {
	DefaultLocalProxyHTTPMiddleware

	// certReissuer is used to reissue a client certificate for a Kubernetes cluster if existing cert expired.
	certReissuer KubeCertReissuer
	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification. Defaults to real clock if unspecified.
	clock clockwork.Clock
	// headless controls whether proxy is working in headless login mode.
	headless bool

	logger       *slog.Logger
	closeContext context.Context

	// isCertReissuingRunning is used to only ever have one concurrent cert reissuing session requiring user input.
	isCertReissuingRunning atomic.Bool

	certsMu sync.RWMutex
	// certs is a map by cluster name of Kubernetes client certs.
	certs KubeClientCerts
}

type KubeMiddlewareConfig struct {
	Certs        KubeClientCerts
	CertReissuer KubeCertReissuer
	Headless     bool
	Clock        clockwork.Clock
	Logger       *slog.Logger
	CloseContext context.Context
}

// NewKubeMiddleware creates a new KubeMiddleware.
func NewKubeMiddleware(cfg KubeMiddlewareConfig) LocalProxyHTTPMiddleware {
	return &KubeMiddleware{
		certs:        cfg.Certs,
		certReissuer: cfg.CertReissuer,
		headless:     cfg.Headless,
		clock:        cfg.Clock,
		logger:       cfg.Logger,
		closeContext: cfg.CloseContext,
	}
}

// CheckAndSetDefaults checks configuration validity and sets defaults
func (m *KubeMiddleware) CheckAndSetDefaults() error {
	if m.certs == nil {
		return trace.BadParameter("missing certs")
	}
	if m.clock == nil {
		m.clock = clockwork.NewRealClock()
	}
	if m.logger == nil {
		m.logger = slog.With(teleport.ComponentKey, "local_proxy_kube")
	}
	if m.closeContext == nil {
		return trace.BadParameter("missing close context")
	}
	return nil
}

func initKubeCodecs() serializer.CodecFactory {
	kubeScheme := runtime.NewScheme()
	// It manually registers support for `metav1.Table` because go-client does not
	// support it but `kubectl` calls require support for it.
	utilruntime.Must(metav1.AddMetaToScheme(kubeScheme))
	utilruntime.Must(scheme.AddToScheme(kubeScheme))

	return serializer.NewCodecFactory(kubeScheme)
}

func writeKubeError(ctx context.Context, rw http.ResponseWriter, kubeError *apierrors.StatusError, logger *slog.Logger) {
	kubeCodecs := initKubeCodecs()
	status := kubeError.Status()
	errorBytes, err := runtime.Encode(kubeCodecs.LegacyCodec(), &status)
	if err != nil {
		logger.WarnContext(ctx, "Failed to encode Kube status error", "error", err)
		trace.WriteError(rw, trace.Wrap(kubeError))
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(int(status.Code))

	if _, err := rw.Write(errorBytes); err != nil {
		logger.WarnContext(ctx, "Failed to write Kube error", "error", err)
	}
}

// ClearCerts clears the middleware certs.
// It will try to reissue them when a new request comes in.
func (m *KubeMiddleware) ClearCerts() {
	m.certsMu.Lock()
	defer m.certsMu.Unlock()
	clear(m.certs)
}

// HandleRequest checks if middleware has valid certificate for this request and
// reissues it if needed. In case of reissuing error we write directly to the response and return true,
// so caller won't continue processing the request.
func (m *KubeMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	cert, err := m.getCertForRequest(req)
	// If the cert is cleared using m.ClearCerts(), it won't be found.
	// This forces the middleware to issue a new cert on a new request.
	// This is used in access requests in Connect where we want to refresh certs without closing the proxy.
	if err != nil && !trace.IsNotFound(err) {
		return false
	}

	err = m.reissueCertIfExpired(req.Context(), cert, req.TLS.ServerName)
	if err != nil {
		// If user input is required we return an error that will try to get user attention to the local proxy
		if errors.Is(err, ErrUserInputRequired) {
			writeKubeError(req.Context(), rw, &apierrors.StatusError{
				ErrStatus: metav1.Status{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Status",
						APIVersion: "v1",
					},
					Status:  metav1.StatusFailure,
					Code:    http.StatusGatewayTimeout,
					Reason:  metav1.StatusReasonTimeout,
					Message: "Local Teleport Kube proxy requires user input to continue",
				},
			}, m.logger)
			return true
		}
		m.logger.WarnContext(req.Context(), "Failed to reissue certificate for server", "server", req.TLS.ServerName)
		trace.WriteError(rw, trace.Wrap(err))
		return true
	}

	return false
}

func (m *KubeMiddleware) getCertForRequest(req *http.Request) (tls.Certificate, error) {
	if req.TLS == nil {
		return tls.Certificate{}, trace.BadParameter("expect a TLS request")
	}

	m.certsMu.RLock()
	cert, ok := m.certs[req.TLS.ServerName]
	m.certsMu.RUnlock()
	if !ok {
		return tls.Certificate{}, trace.NotFound("no client cert found for %v", req.TLS.ServerName)
	}

	return cert, nil
}

// OverwriteClientCerts overwrites the client certs used for upstream connection.
func (m *KubeMiddleware) OverwriteClientCerts(req *http.Request) ([]tls.Certificate, error) {
	cert, err := m.getCertForRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []tls.Certificate{cert}, nil
}

// ErrUserInputRequired returned when user's input required to relogin and/or reissue new certificate.
var ErrUserInputRequired = errors.New("user input required")

// reissueCertIfExpired checks if provided certificate has expired and reissues it if needed and replaces in the middleware certs.
// serverName has a form of <hex-encoded-kube-cluster>.<teleport-cluster>.
func (m *KubeMiddleware) reissueCertIfExpired(ctx context.Context, cert tls.Certificate, serverName string) error {
	needsReissue := false
	if len(cert.Certificate) == 0 {
		m.logger.InfoContext(ctx, "missing TLS certificate, attempting to reissue a new one")
		needsReissue = true
	} else {
		x509Cert, err := utils.TLSCertLeaf(cert)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := utils.VerifyCertificateExpiry(x509Cert, m.clock); err != nil {
			needsReissue = true
		}
	}
	if !needsReissue {
		return nil
	}

	if m.certReissuer == nil {
		return trace.BadParameter("can't reissue proxy certificate - reissuer is not available")
	}
	teleportCluster := common.TeleportClusterFromKubeLocalProxySNI(serverName)
	if teleportCluster == "" {
		return trace.BadParameter("can't reissue proxy certificate - teleport cluster is empty")
	}
	kubeCluster, err := common.KubeClusterFromKubeLocalProxySNI(serverName)
	if err != nil {
		return trace.Wrap(err, "can't reissue proxy certificate - kube cluster name is invalid")
	}
	if kubeCluster == "" {
		return trace.BadParameter("can't reissue proxy certificate - kube cluster is empty")
	}

	errCh := make(chan error, 1)
	// We start cert reissuing (with relogin if required) only if it's not running already.
	// After that it will run until user gives required input.
	// User requests will return error notifying about required user input while reissuing is running.
	if m.isCertReissuingRunning.CompareAndSwap(false, true) {
		go func() {
			defer m.isCertReissuingRunning.Store(false)

			newCert, err := m.certReissuer(m.closeContext, teleportCluster, kubeCluster)
			if err == nil {
				m.certsMu.Lock()
				m.certs[serverName] = newCert
				m.certsMu.Unlock()
			}
			errCh <- err
		}()
	} else {
		return trace.Wrap(ErrUserInputRequired)
	}

	reissueClientWait := certReissueClientWait
	if m.headless {
		reissueClientWait = certReissueClientWaitHeadless
	}

	select {
	case <-time.After(reissueClientWait):
		return trace.Wrap(ErrUserInputRequired)
	case err := <-errCh:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// NewKubeListener creates a listener for kube local proxy.
func NewKubeListener(casByTeleportCluster map[string]tls.Certificate) (net.Listener, error) {
	configs := make(map[string]*tls.Config)
	for teleportCluster, ca := range casByTeleportCluster {
		caLeaf, err := utils.TLSCertLeaf(ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ca.Leaf = caLeaf

		// Server and client are using the same certs.
		clientCAs := x509.NewCertPool()
		clientCAs.AddCert(caLeaf)

		configs[teleportCluster] = &tls.Config{
			Certificates: []tls.Certificate{ca},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    clientCAs,
		}
	}
	listener, err := tls.Listen("tcp", "localhost:0", &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			config, ok := configs[common.TeleportClusterFromKubeLocalProxySNI(hello.ServerName)]
			if !ok {
				return nil, trace.BadParameter("unknown Teleport cluster or invalid TLS server name %v", hello.ServerName)
			}
			return config, nil
		},
	})
	return listener, trace.Wrap(err)
}

// KubeForwardProxyConfig is the config for making kube forward proxy.
type KubeForwardProxyConfig struct {
	// CloseContext is the close context.
	CloseContext context.Context
	// ListenPort is the localhost port to listen.
	ListenPort string
	// Listener is the listener for the forward proxy. A listener is created
	// from ListenPort if Listener is not provided.
	Listener net.Listener
	// ForwardAddr is the target address the requests get forwarded to.
	ForwardAddr string
}

// CheckAndSetDefaults checks and sets default config values.
func (c *KubeForwardProxyConfig) CheckAndSetDefaults() error {
	if c.ForwardAddr == "" {
		return trace.BadParameter("missing forward address")
	}
	if c.CloseContext == nil {
		c.CloseContext = context.Background()
	}
	if c.Listener == nil {
		if c.ListenPort == "" {
			c.ListenPort = "0"
		}

		listener, err := net.Listen("tcp", "localhost:"+c.ListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		c.Listener = listener
	}
	return nil
}

// NewKubeForwardProxy creates a forward proxy for kube access.
func NewKubeForwardProxy(config KubeForwardProxyConfig) (*ForwardProxy, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	fp, err := NewForwardProxy(ForwardProxyConfig{
		Listener:     config.Listener,
		CloseContext: config.CloseContext,
		Handlers: []ConnectRequestHandler{
			NewForwardToHostHandler(ForwardToHostHandlerConfig{
				MatchFunc: MatchAllRequests,
				Host:      config.ForwardAddr,
			}),
		},
	})
	if err != nil {
		return nil, trace.NewAggregate(config.Listener.Close(), err)
	}
	return fp, nil
}

// CreateKubeLocalCAs generate local CAs used for kube local proxy with provided key.
func CreateKubeLocalCAs(key *keys.PrivateKey, teleportClusters []string) (map[string]tls.Certificate, error) {
	cas := make(map[string]tls.Certificate)
	for _, teleportCluster := range teleportClusters {
		ca, err := createLocalCA(key, time.Now().Add(defaults.CATTL), common.KubeLocalProxyWildcardDomain(teleportCluster))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cas[teleportCluster] = ca
	}
	return cas, nil
}

func createLocalCA(key *keys.PrivateKey, validUntil time.Time, dnsNames ...string) (tls.Certificate, error) {
	cert, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Entity: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Teleport"},
		},
		Signer:      key,
		DNSNames:    dnsNames,
		IPAddresses: []net.IP{net.ParseIP(defaults.Localhost)},
		TTL:         time.Until(validUntil),
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	tlsCert, err := keys.X509KeyPair(cert, key.PrivateKeyPEM())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return tlsCert, nil
}

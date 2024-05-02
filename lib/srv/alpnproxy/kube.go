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
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
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

	logger logrus.FieldLogger

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
	Logger       logrus.FieldLogger
}

// NewKubeMiddleware creates a new KubeMiddleware.
func NewKubeMiddleware(cfg KubeMiddlewareConfig) LocalProxyHTTPMiddleware {
	return &KubeMiddleware{
		certs:        cfg.Certs,
		certReissuer: cfg.CertReissuer,
		headless:     cfg.Headless,
		clock:        cfg.Clock,
		logger:       cfg.Logger,
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
		m.logger = logrus.WithField(teleport.ComponentKey, "local_proxy_kube")
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

func writeKubeError(rw http.ResponseWriter, kubeError *apierrors.StatusError, logger logrus.FieldLogger) {
	kubeCodecs := initKubeCodecs()
	status := kubeError.Status()
	errorBytes, err := runtime.Encode(kubeCodecs.LegacyCodec(), &status)
	if err != nil {
		logger.Warnf("Failed to encode Kube status error: %v.", err)
		trace.WriteError(rw, trace.Wrap(kubeError))
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(int(status.Code))

	if _, err := rw.Write(errorBytes); err != nil {
		logger.Warnf("Failed to write Kube error: %v.", err)
	}
}

// HandleRequest checks if middleware has valid certificate for this request and
// reissues it if needed. In case of reissuing error we write directly to the response and return true,
// so caller won't continue processing the request.
func (m *KubeMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	cert, err := m.getCertForRequest(req)
	if err != nil {
		return false
	}

	err = m.reissueCertIfExpired(req.Context(), cert, req.TLS.ServerName)
	if err != nil {
		// If user input is required we return an error that will try to get user attention to the local proxy
		if errors.Is(err, ErrUserInputRequired) {
			writeKubeError(rw, &apierrors.StatusError{
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
		m.logger.WithError(err).Warnf("Failed to reissue certificate for server %v", req.TLS.ServerName)
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
func (m *KubeMiddleware) reissueCertIfExpired(ctx context.Context, cert tls.Certificate, serverName string) error {
	x509Cert, err := utils.TLSCertLeaf(cert)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := utils.VerifyCertificateExpiry(x509Cert, m.clock); err == nil {
		return nil
	}

	if m.certReissuer == nil {
		return trace.BadParameter("can't reissue expired proxy certificate - reissuer is not available")
	}

	// If certificate has expired we try to reissue it.
	identity, err := tlsca.FromSubject(x509Cert.Subject, x509Cert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}

	errCh := make(chan error, 1)
	// We start cert reissuing (with relogin if required) only if it's not running already.
	// After that it will run until user gives required input.
	// User requests will return error notifying about required user input while reissuing is running.
	if m.isCertReissuingRunning.CompareAndSwap(false, true) {
		go func() {
			defer m.isCertReissuingRunning.Store(false)

			cluster := identity.TeleportCluster
			if identity.RouteToCluster != "" {
				cluster = identity.RouteToCluster
			}
			newCert, err := m.certReissuer(ctx, cluster, identity.KubernetesCluster)
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

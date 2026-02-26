package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

const (
	saTokenPath  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	saCACertPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// K8sProxy sits between Headlamp and the real Kubernetes API. It validates
// the internal HMAC token from Headlamp requests, and forwards to the real
// K8s API with impersonation headers using the pod's ServiceAccount.
type K8sProxy struct {
	proxy  *httputil.ReverseProxy
	signer *TokenSigner
}

// NewK8sProxy creates the K8s API proxy. It reads the real API server address
// from the standard environment variables and the ServiceAccount credentials
// from the mounted secret.
func NewK8sProxy(signer *TokenSigner) (*K8sProxy, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be set")
	}

	// IPv6 addresses need brackets.
	if strings.ContainsRune(host, ':') {
		host = "[" + host + "]"
	}

	target, err := url.Parse(fmt.Sprintf("https://%s:%s", host, port))
	if err != nil {
		return nil, fmt.Errorf("parsing K8s API address: %w", err)
	}

	// Load the cluster CA to verify the real API server.
	caCert, err := os.ReadFile(saCACertPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert: %w", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: caCertPool,
		},
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = transport

	return &K8sProxy{
		proxy:  proxy,
		signer: signer,
	}, nil
}

func (kp *K8sProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read the ServiceAccount token on each request (it may be rotated).
	saToken, err := os.ReadFile(saTokenPath)
	if err != nil {
		slog.Error("reading SA token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Check for the internal HMAC token. The front proxy passes it via
	// X-Auth-Token header which survives Headlamp's internal reverse proxy.
	// Also check the Authorization bearer as a fallback.
	token := r.Header.Get("X-Auth-Token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		token = strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			token = "" // not a Bearer token
		}
	}

	if token != "" {
		claims, err := kp.signer.Validate(token)
		if err == nil {
			// Valid internal token — impersonate the user.
			r.Header.Del("X-Auth-Token")
			r.Header.Set("Authorization", "Bearer "+string(saToken))
			r.Header.Set("Impersonate-User", claims.Username)
			for _, g := range claims.Groups {
				r.Header.Add("Impersonate-Group", g)
			}
			slog.Info("impersonating", "user", claims.Username, "groups", claims.Groups, "path", r.URL.Path)
			kp.proxy.ServeHTTP(w, r)
			return
		}
		slog.Warn("HMAC validation failed, falling through", "error", err, "path", r.URL.Path)
	} else {
		slog.Info("no auth token, using SA", "path", r.URL.Path)
	}

	// No token or non-HMAC token: forward using the SA token for
	// Headlamp's own operations (not user-scoped).
	r.Header.Set("Authorization", "Bearer "+string(saToken))
	kp.proxy.ServeHTTP(w, r)
}

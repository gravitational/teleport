// headlamp-auth-proxy bridges Teleport app access identity to Kubernetes
// RBAC via impersonation. It runs as a sidecar alongside Headlamp with two
// proxy roles:
//
//  1. Front proxy (:4466) — decodes the Teleport JWT, mints an internal
//     HMAC token encoding the user identity, injects it as a Headlamp session
//     cookie, and forwards to Headlamp (:4467).
//
//  2. K8s API proxy (:6443) — receives requests from Headlamp bearing the
//     HMAC token, validates it, and forwards to the real K8s API with
//     Impersonate-User/Impersonate-Group headers using the pod's
//     ServiceAccount credentials.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

func main() {
	var (
		headlampAddr   = flag.String("headlamp-addr", "127.0.0.1:4467", "Headlamp backend address")
		listenHTTP     = flag.String("listen-http", ":4466", "Front proxy listen address")
		listenK8s      = flag.String("listen-k8s", "127.0.0.1:6443", "K8s API proxy listen address")
		proxyAddr      = flag.String("proxy-addr", "", "Teleport proxy URL for JWKS verification (overrides --teleport-config)")
		teleportConfig = flag.String("teleport-config", "", "Path to teleport.yaml to read proxy_server from")
		groupsClaim    = flag.String("groups-claim", "roles", "JWT claim to map to K8s groups (roles or traits.<key>)")
		cookieName     = flag.String("cookie-name", "headlamp-auth-main.0", "Headlamp session cookie name (format: headlamp-auth-{cluster}.{chunk})")
		tokenTTL       = flag.Duration("token-ttl", 5*time.Minute, "Internal HMAC token TTL")
	)
	flag.Parse()

	if *proxyAddr == "" && *teleportConfig != "" {
		addr, err := proxyAddrFromConfig(*teleportConfig)
		if err != nil {
			slog.Error("reading proxy address from teleport config", "error", err)
			os.Exit(1)
		}
		*proxyAddr = addr
	}
	if *proxyAddr == "" {
		slog.Error("either --proxy-addr or --teleport-config is required")
		os.Exit(1)
	}

	// Generate an ephemeral HMAC key. Tokens are invalidated on pod restart.
	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		slog.Error("generating HMAC key", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	verifier, err := NewJWKSVerifier(*proxyAddr)
	if err != nil {
		slog.Error("initializing JWKS verifier", "error", err)
		os.Exit(1)
	}
	go verifier.RefreshLoop(ctx, 5*time.Minute)

	signer := &TokenSigner{
		key: hmacKey,
		ttl: *tokenTTL,
	}

	frontProxy, err := NewFrontProxy(*headlampAddr, verifier, signer, *groupsClaim, *cookieName)
	if err != nil {
		slog.Error("creating front proxy", "error", err)
		os.Exit(1)
	}

	k8sProxy, err := NewK8sProxy(signer)
	if err != nil {
		slog.Error("creating K8s proxy", "error", err)
		os.Exit(1)
	}

	frontServer := &http.Server{
		Addr:    *listenHTTP,
		Handler: frontProxy,
	}
	k8sServer := &http.Server{
		Addr:    *listenK8s,
		Handler: k8sProxy,
	}

	go func() {
		slog.Info("front proxy listening", "addr", *listenHTTP)
		if err := frontServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("front proxy", "error", err)
			cancel()
		}
	}()

	go func() {
		slog.Info("k8s proxy listening", "addr", *listenK8s)
		if err := k8sServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("k8s proxy", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	frontServer.Shutdown(shutdownCtx)
	k8sServer.Shutdown(shutdownCtx)
}

// proxyAddrFromConfig reads the Teleport proxy address from a teleport.yaml
// config file (the teleport-agent ConfigMap) and returns it as an https URL.
func proxyAddrFromConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg struct {
		Teleport struct {
			ProxyServer string `yaml:"proxy_server"`
		} `yaml:"teleport"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.Teleport.ProxyServer == "" {
		return "", fmt.Errorf("teleport.proxy_server not set in %s", path)
	}

	return "https://" + cfg.Teleport.ProxyServer, nil
}

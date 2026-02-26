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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var (
		headlampAddr = flag.String("headlamp-addr", "127.0.0.1:4467", "Headlamp backend address")
		listenHTTP   = flag.String("listen-http", ":4466", "Front proxy listen address")
		listenK8s    = flag.String("listen-k8s", "127.0.0.1:6443", "K8s API proxy listen address")
		groupsClaim  = flag.String("groups-claim", "roles", "JWT claim to map to K8s groups (roles or traits.<key>)")
		cookieName   = flag.String("cookie-name", "headlamp_main_token", "Headlamp session cookie name")
		tokenTTL     = flag.Duration("token-ttl", 5*time.Minute, "Internal HMAC token TTL")
	)
	flag.Parse()

	// Generate an ephemeral HMAC key. Tokens are invalidated on pod restart.
	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		slog.Error("generating HMAC key", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	signer := &TokenSigner{
		key: hmacKey,
		ttl: *tokenTTL,
	}

	frontProxy, err := NewFrontProxy(*headlampAddr, signer, *groupsClaim, *cookieName)
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

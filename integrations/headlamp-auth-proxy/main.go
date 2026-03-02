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
		proxyAddr    = flag.String("proxy-addr", "", "Teleport proxy URL for JWKS verification (e.g. https://teleport.example.com)")
		groupsClaim  = flag.String("groups-claim", "roles", "JWT claim to map to K8s groups (roles or traits.<key>)")
		cookieName   = flag.String("cookie-name", "headlamp-auth-main.0", "Headlamp session cookie name (format: headlamp-auth-{cluster}.{chunk})")
		tokenTTL     = flag.Duration("token-ttl", 5*time.Minute, "Internal HMAC token TTL")
	)
	flag.Parse()

	if *proxyAddr == "" {
		*proxyAddr = os.Getenv("TELEPORT_PROXY_ADDR")
	}
	if *proxyAddr == "" {
		slog.Error("--proxy-addr flag or TELEPORT_PROXY_ADDR env var is required")
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

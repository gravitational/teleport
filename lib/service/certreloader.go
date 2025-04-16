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

package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// CertReloaderConfig contains the configuration of the certificate reloader.
type CertReloaderConfig struct {
	// KeyPairs are the key and certificate pairs that the proxy will load.
	KeyPairs []servicecfg.KeyPairPath
	// KeyPairsReloadInterval is the interval between attempts to reload
	// x509 key pairs. If set to 0, then periodic reloading is disabled.
	KeyPairsReloadInterval time.Duration
}

// CertReloader periodically reloads a list of cert key-pair paths.
// This allows new certificates to be used without a full reload of Teleport.
type CertReloader struct {
	logger *slog.Logger
	// cfg is the certificate reloader configuration.
	cfg CertReloaderConfig

	// certificates is the list of certificates loaded.
	certificates []tls.Certificate
	// mu protects the list of certificates.
	mu sync.RWMutex
}

// NewCertReloader initializes a new certificate reloader.
func NewCertReloader(cfg CertReloaderConfig) *CertReloader {
	return &CertReloader{
		logger: slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentProxy, "certreloader")),
		cfg:    cfg,
	}
}

// Run tries to load certificates and then spawns the certificate reloader.
func (c *CertReloader) Run(ctx context.Context) error {
	// Synchronously load initially configured certificates.
	if err := c.loadCertificates(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Do not reload certificates if the interval is 0 (the default).
	if c.cfg.KeyPairsReloadInterval == 0 {
		return nil
	}

	// Spawn the certificate reloader.
	go func() {
		c.logger.InfoContext(ctx, "Starting periodic reloading of certificate key pairs", "reload_interval", c.cfg.KeyPairsReloadInterval)
		defer func() {
			c.logger.InfoContext(ctx, "Stopped periodic reloading of certificate key pairs")
		}()

		t := time.NewTicker(c.cfg.KeyPairsReloadInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := c.loadCertificates(ctx); err != nil {
					c.logger.WarnContext(ctx, "Failed to load certificates", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// loadCertificates loads certificate keys pairs.
// It returns an error if any of the certificate key pairs fails to load.
// If any of the key pairs fails to load, none of the certificates are updated.
func (c *CertReloader) loadCertificates(ctx context.Context) error {
	certs := make([]tls.Certificate, 0, len(c.cfg.KeyPairs))
	for _, pair := range c.cfg.KeyPairs {
		c.logger.DebugContext(ctx, "Loading TLS certificate",
			"public_key", pair.Certificate,
			"private_key", pair.PrivateKey,
		)

		certificate, err := tls.LoadX509KeyPair(pair.Certificate, pair.PrivateKey)
		if err != nil {
			// If one certificate fails to load, then no certificate is updated.
			return trace.WrapWithMessage(err, "TLS certificate %v and key %v failed to load", pair.Certificate, pair.PrivateKey)
		}

		// Parse the end entity cert and add it to certificate.Leaf.
		// With this, the SupportsCertificate call doesn't have to
		// parse it on every GetCertificate call.
		leaf, err := x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			// If one certificate fails to load, then no certificate is updated.
			return trace.WrapWithMessage(err, "TLS certificate %v and key %v failed to be parsed", pair.Certificate, pair.PrivateKey)
		}
		certificate.Leaf = leaf

		certs = append(certs, certificate)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.certificates = certs

	return nil
}

// GetCertificate is compatible with tls.Config.GetCertificate, allowing
// the CertReloader to be a source of certificates for a TLS listener.
// Certificate selection logic is the same as getCertificate in crypto/tls:
// https://github.com/golang/go/tree/f64c2a2ce5dc859315047184e310879dcf747d53/src/crypto/tls/common.go#L1075-L1117
func (c *CertReloader) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.certificates) == 0 {
		return nil, trace.NotFound("found no certificates in cert reloader")
	}

	if len(c.certificates) == 1 {
		// There's only one choice, so no point doing any work.
		return &c.certificates[0], nil
	}

	// If there's more than one choice, select the first certificate that matches.
	for _, cert := range c.certificates {
		if err := clientHello.SupportsCertificate(&cert); err == nil {
			return &cert, nil
		}
	}
	// If nothing matches, return the first certificate.
	return &c.certificates[0], nil
}

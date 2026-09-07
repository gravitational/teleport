/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package config

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/observability/tracing"
)

func TestTracingConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	junkPath := filepath.Join(dir, "junk.pem")
	require.NoError(t, os.WriteFile(certPath, fixtures.LocalhostCert, 0o600))
	require.NoError(t, os.WriteFile(keyPath, fixtures.LocalhostKey, 0o600))
	require.NoError(t, os.WriteFile(junkPath, []byte("not a pem"), 0o600))
	missingPath := filepath.Join(dir, "missing.pem")

	tests := []struct {
		name string
		in   TracingConfig
		// wantErr is matched against CheckAndSetDefaults; if empty, the
		// TraceConfig assertions below run.
		wantErr          string
		wantTraceCfgErr  string
		wantSamplingRate float64
		wantTLS          bool
		wantRootCAs      bool
		wantCerts        int
	}{
		{
			name: "disabled ignores invalid fields",
			in:   TracingConfig{SamplingRatePerMillion: new(-1)},
		},
		{
			name:             "grpc is plaintext without tls material",
			in:               TracingConfig{Enabled: true, ExporterURL: "grpc://collector:4317"},
			wantSamplingRate: 1.0,
		},
		{
			name:             "bare host port is grpc",
			in:               TracingConfig{Enabled: true, ExporterURL: "collector:4317"},
			wantSamplingRate: 1.0,
		},
		{
			name:             "https uses tls with system roots by default",
			in:               TracingConfig{Enabled: true, ExporterURL: "https://collector:4318"},
			wantSamplingRate: 1.0,
			wantTLS:          true,
		},
		{
			name:             "http is plaintext",
			in:               TracingConfig{Enabled: true, ExporterURL: "http://collector:4318"},
			wantSamplingRate: 1.0,
		},
		{
			name:             "file has no tls",
			in:               TracingConfig{Enabled: true, ExporterURL: "file:///var/lib/tbot/traces"},
			wantSamplingRate: 1.0,
		},
		{
			name: "sampling rate converted",
			in: TracingConfig{
				Enabled:                true,
				ExporterURL:            "grpc://collector:4317",
				SamplingRatePerMillion: new(250_000),
			},
			wantSamplingRate: 0.25,
		},
		{
			name: "tls material loaded",
			in: TracingConfig{
				Enabled:     true,
				ExporterURL: "grpc://collector:4317",
				CACerts:     []string{certPath},
				KeyPairs:    []KeyPair{{PrivateKey: keyPath, Certificate: certPath}},
			},
			wantSamplingRate: 1.0,
			wantTLS:          true,
			wantRootCAs:      true,
			wantCerts:        1,
		},
		{
			name: "ca_certs rejected for http",
			in: TracingConfig{
				Enabled:     true,
				ExporterURL: "http://collector:4318",
				CACerts:     []string{certPath},
			},
			wantErr: "ca_certs: cannot be used with the http scheme",
		},
		{
			name: "keypairs rejected for file",
			in: TracingConfig{
				Enabled:     true,
				ExporterURL: "file:///var/lib/tbot/traces",
				KeyPairs:    []KeyPair{{PrivateKey: keyPath, Certificate: certPath}},
			},
			wantErr: "keypairs: cannot be used with the file scheme",
		},
		{
			name:    "enabled without exporter",
			in:      TracingConfig{Enabled: true},
			wantErr: "exporter_url: must be specified",
		},
		{
			name:    "unsupported scheme",
			in:      TracingConfig{Enabled: true, ExporterURL: "udp://collector:4317"},
			wantErr: "exporter_url: must use one of",
		},
		{
			name: "sampling rate too high",
			in: TracingConfig{
				Enabled:                true,
				ExporterURL:            "grpc://collector:4317",
				SamplingRatePerMillion: new(1_000_001),
			},
			wantErr: "sampling_rate_per_million: must be between",
		},
		{
			name: "missing key",
			in: TracingConfig{
				Enabled:     true,
				ExporterURL: "grpc://collector:4317",
				KeyPairs:    []KeyPair{{PrivateKey: missingPath, Certificate: certPath}},
			},
			wantErr: "keypairs: private key does not exist",
		},
		{
			name: "missing ca",
			in: TracingConfig{
				Enabled:     true,
				ExporterURL: "grpc://collector:4317",
				CACerts:     []string{missingPath},
			},
			wantErr: "ca_certs: file does not exist",
		},
		{
			name: "unparseable ca",
			in: TracingConfig{
				Enabled:     true,
				ExporterURL: "grpc://collector:4317",
				CACerts:     []string{junkPath},
			},
			wantTraceCfgErr: "parsing tracing CA certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.CheckAndSetDefaults()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if !tt.in.Enabled {
				return
			}

			cfg, err := tt.in.TraceConfig()
			if tt.wantTraceCfgErr != "" {
				require.ErrorContains(t, err, tt.wantTraceCfgErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "tbot", cfg.Service)
			require.Equal(t, tt.in.ExporterURL, cfg.ExporterURL)
			require.InDelta(t, tt.wantSamplingRate, cfg.SamplingRate, 0)
			if !tt.wantTLS {
				require.Nil(t, cfg.TLSConfig)
				return
			}
			require.Equal(t, tt.wantRootCAs, cfg.TLSConfig.RootCAs != nil)
			require.Len(t, cfg.TLSConfig.Certificates, tt.wantCerts)
		})
	}
}

// TestTracingConfig_MTLSExport proves the loaded TLS material works against a
// collector that requires client certificates.
func TestTracingConfig_MTLSExport(t *testing.T) {
	// Not parallel: NewTraceProvider registers the global provider.
	ctx := t.Context()
	dir := t.TempDir()

	// One self-signed cert plays CA, collector cert and client cert.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "tracing test"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	serverCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(certPEM))
	collector, err := tracing.NewCollector(tracing.CollectorConfig{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    pool,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, collector.Shutdown(shutdownCtx))
	})
	go func() {
		_ = collector.Start()
	}()

	cfg := TracingConfig{
		Enabled:     true,
		ExporterURL: collector.GRPCAddr(),
		CACerts:     []string{certPath},
		KeyPairs:    []KeyPair{{PrivateKey: keyPath, Certificate: certPath}},
	}
	require.NoError(t, cfg.CheckAndSetDefaults())
	traceCfg, err := cfg.TraceConfig()
	require.NoError(t, err)
	provider, err := tracing.NewTraceProvider(ctx, *traceCfg)
	require.NoError(t, err)

	_, span := provider.Tracer("test").Start(ctx, "test-span")
	span.End()
	require.NoError(t, provider.Shutdown(ctx))

	var names []string
	for _, ss := range collector.Spans() {
		for _, s := range ss.Spans {
			names = append(names, s.Name)
		}
	}
	require.Equal(t, []string{"test-span"}, names)
}

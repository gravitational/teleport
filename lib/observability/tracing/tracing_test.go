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

package tracing

import (
	"bytes"
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/cryptopatch"
)

func generateTLSCertificate() (tls.Certificate, error) {
	priv, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Test CA"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv6loopback, net.IPv4(127, 0, 0, 1)},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	var certificateBuffer bytes.Buffer
	if err := pem.Encode(&certificateBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return tls.Certificate{}, err
	}
	privDERBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	var privBuffer bytes.Buffer
	if err := pem.Encode(&privBuffer, &pem.Block{Type: "PRIVATE KEY", Bytes: privDERBytes}); err != nil {
		return tls.Certificate{}, err
	}

	tlsCertificate, err := tls.X509KeyPair(certificateBuffer.Bytes(), privBuffer.Bytes())
	if err != nil {
		return tls.Certificate{}, err
	}
	return tlsCertificate, nil
}

func TestNewClient(t *testing.T) {
	t.Parallel()
	c, err := NewCollector(CollectorConfig{})
	require.NoError(t, err)

	cfg := Config{
		Service:     "test",
		ExporterURL: c.GRPCAddr(),
		DialTimeout: time.Second,
	}

	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		require.NoError(t, c.Shutdown(shutdownCtx))
	})
	go func() {
		c.Start()
	}()

	// NewClient shouldn't fail - it won't attempt to connect to the Collector
	clt, err := NewClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, clt)

	// Starting the client should be successful when the Collector is up
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, clt.Start(ctx))

	// NewStartedClient will dial the collector, if everything is OK
	// then it should return a valid client
	clt, err = NewStartedClient(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, clt)

	// Stop the Collector
	require.NoError(t, c.Shutdown(context.Background()))

	// NewClient shouldn't fail - it won't attempt to connect to the Collector
	clt, err = NewClient(cfg)
	require.NoError(t, err, "NewClient failed even though it doesn't dial the Collector")
	require.NotNil(t, clt)

	// Starting clients when the Collector is offline is allowed since no IO occurs
	// until issuing an RPC.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	require.NoError(t, clt.Start(context.Background()))
	require.Error(t, clt.UploadTraces(ctx2, nil))

	// NewStartedClient won't dial the collector, if the Collector is offline
	// then it shouldn't be detected until an RPC is issued.
	clt, err = NewStartedClient(context.Background(), cfg)
	require.NoError(t, err, "NewStartedClient was successful dialing an offline Collector")
	require.NotNil(t, clt)
	ctx3, cancel3 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel3()
	require.Error(t, clt.UploadTraces(ctx3, nil))
}

func TestNewExporter(t *testing.T) {
	t.Parallel()
	c, err := NewCollector(CollectorConfig{})
	require.NoError(t, err)

	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		require.NoError(t, c.Shutdown(shutdownCtx))
	})
	go func() {
		c.Start()
	}()

	cases := []struct {
		name              string
		config            Config
		errAssertion      require.ErrorAssertionFunc
		exporterAssertion require.ValueAssertionFunc
	}{
		{
			name: "invalid config",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err, i...)
				require.True(t, trace.IsBadParameter(err), i...)
			},
			exporterAssertion: require.Nil,
		},
		{
			name: "invalid exporter url",
			config: Config{
				Service:     "test",
				ExporterURL: "tcp://localhost:123",
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err, i...)
				require.True(t, trace.IsBadParameter(err), i...)
			},
			exporterAssertion: require.Nil,
		},
		{
			name: "successful explicit grpc exporter",
			config: Config{
				Service:     "test",
				ExporterURL: c.GRPCAddr(),
				DialTimeout: time.Second,
			},
			errAssertion:      require.NoError,
			exporterAssertion: require.NotNil,
		},
		{
			name: "successful inferred grpc exporter",
			config: Config{
				Service:     "test",
				ExporterURL: c.GRPCAddr()[len("grpc://"):],
				DialTimeout: time.Second,
			},
			errAssertion:      require.NoError,
			exporterAssertion: require.NotNil,
		},
		{
			name: "successful http exporter",
			config: Config{
				Service:     "test",
				ExporterURL: c.HTTPAddr(),
				DialTimeout: time.Second,
			},
			errAssertion:      require.NoError,
			exporterAssertion: require.NotNil,
		},
		{
			name: "file exporter",
			config: Config{
				Service:     "test",
				ExporterURL: "file://" + t.TempDir(),
			},
			errAssertion:      require.NoError,
			exporterAssertion: require.NotNil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := NewExporter(context.Background(), tt.config)
			if exporter != nil {
				t.Cleanup(func() { require.NoError(t, exporter.Shutdown(context.Background())) })
			}
			tt.errAssertion(t, err)
			tt.exporterAssertion(t, exporter)
		})
	}
}

func TestTraceProvider(t *testing.T) {
	const spansCreated = 4

	tlsCertificate, err := generateTLSCertificate()
	require.NoError(t, err)
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCertificate},
		InsecureSkipVerify: true,
	}

	cases := []struct {
		name              string
		config            func(c *Collector) Config
		errAssertion      require.ErrorAssertionFunc
		providerAssertion require.ValueAssertionFunc
		collectedLen      int
		tlsConfig         *tls.Config
	}{
		{
			name: "not sampling prevents exporting",
			config: func(c *Collector) Config {
				return Config{
					Service:     "test",
					ExporterURL: c.GRPCAddr(),
					DialTimeout: time.Second,
					TLSConfig:   c.ClientTLSConfig(),
				}
			},
			errAssertion:      require.NoError,
			providerAssertion: require.NotNil,
			collectedLen:      0,
		},
		{
			name: "spans exported with gRPC+TLS",
			config: func(c *Collector) Config {
				return Config{
					Service:      "test",
					SamplingRate: 1.0,
					ExporterURL:  c.GRPCAddr(),
					DialTimeout:  time.Second,
					TLSConfig:    c.ClientTLSConfig(),
				}
			},
			errAssertion:      require.NoError,
			providerAssertion: require.NotNil,
			collectedLen:      spansCreated,
			tlsConfig:         tlsConfig,
		},
		{
			name: "spans exported with gRPC",
			config: func(c *Collector) Config {
				return Config{
					Service:      "test",
					SamplingRate: 0.5,
					ExporterURL:  c.GRPCAddr(),
					DialTimeout:  time.Second,
					TLSConfig:    c.ClientTLSConfig(),
				}
			},
			errAssertion:      require.NoError,
			providerAssertion: require.NotNil,
			collectedLen:      spansCreated / 2,
		},
		{
			name: "spans exported with HTTP",
			config: func(c *Collector) Config {
				return Config{
					Service:      "test",
					SamplingRate: 1.0,
					ExporterURL:  c.HTTPAddr(),
					DialTimeout:  time.Second,
					TLSConfig:    c.ClientTLSConfig(),
				}
			},
			errAssertion:      require.NoError,
			providerAssertion: require.NotNil,
			collectedLen:      spansCreated,
		},
		{
			name: "spans exported with HTTPS",
			config: func(c *Collector) Config {
				return Config{
					Service:      "test",
					SamplingRate: 1.0,
					ExporterURL:  c.HTTPSAddr(),
					DialTimeout:  time.Second,
					TLSConfig:    c.ClientTLSConfig(),
				}
			},
			errAssertion:      require.NoError,
			providerAssertion: require.NotNil,
			collectedLen:      spansCreated,
			tlsConfig:         tlsConfig,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			collector, err := NewCollector(CollectorConfig{
				TLSConfig: tt.tlsConfig,
			})
			require.NoError(t, err)

			t.Cleanup(func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer shutdownCancel()
				require.NoError(t, collector.Shutdown(shutdownCtx))
			})
			go func() {
				collector.Start()
			}()

			ctx := context.Background()
			cfg := tt.config(collector)
			provider, err := NewTraceProvider(ctx, cfg)
			tt.errAssertion(t, err)
			tt.providerAssertion(t, provider)

			if err != nil {
				return
			}

			for i := 0; i < spansCreated; i++ {
				_, span := provider.Tracer("test").Start(ctx, fmt.Sprintf("test%d", i))
				span.End()
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, provider.Shutdown(shutdownCtx))
			spans := collector.Spans()
			require.LessOrEqual(t, len(spans), tt.collectedLen)
			require.GreaterOrEqual(t, len(spans), 0)
		})
	}
}

func TestConfig_CheckAndSetDefaults(t *testing.T) {
	cases := []struct {
		name           string
		cfg            Config
		errorAssertion require.ErrorAssertionFunc
		expectedCfg    Config
		expectedURL    *url.URL
	}{
		{
			name: "valid config",
			cfg: Config{
				Service:      "test",
				SamplingRate: 1.0,
				ExporterURL:  "http://localhost:8080",
				DialTimeout:  time.Millisecond,
			},
			errorAssertion: require.NoError,
			expectedCfg: Config{
				Service:      "test",
				ExporterURL:  "http://localhost:8080",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expectedURL: &url.URL{
				Scheme: "http",
				Host:   "localhost:8080",
			},
		},
		{
			name: "invalid service",
			cfg: Config{
				Service:      "",
				SamplingRate: 1.0,
				ExporterURL:  "http://localhost:8080",
			},
			errorAssertion: require.Error,
			expectedCfg: Config{
				Service:      "test",
				ExporterURL:  "http://localhost:8080",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
		},
		{
			name: "invalid exporter url",
			cfg: Config{
				Service:     "test",
				ExporterURL: "",
			},
			errorAssertion: require.Error,
		},
		{
			name: "network address defaults to grpc",
			cfg: Config{
				Service:      "test",
				SamplingRate: 1.0,
				ExporterURL:  "localhost:8080",
				DialTimeout:  time.Millisecond,
			},
			errorAssertion: require.NoError,
			expectedCfg: Config{
				Service:      "test",
				ExporterURL:  "localhost:8080",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expectedURL: &url.URL{
				Scheme: "grpc",
				Host:   "localhost:8080",
			},
		},
		{
			name: "empty scheme defaults to grpc",
			cfg: Config{
				Service:      "test",
				SamplingRate: 1.0,
				ExporterURL:  "exporter.example.com:4317",
				DialTimeout:  time.Millisecond,
			},
			errorAssertion: require.NoError,
			expectedCfg: Config{
				Service:      "test",
				ExporterURL:  "exporter.example.com:4317",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expectedURL: &url.URL{
				Scheme: "grpc",
				Host:   "exporter.example.com:4317",
			},
		},
		{
			name: "timeout defaults to DefaultExporterDialTimeout",
			cfg: Config{
				Service:      "test",
				SamplingRate: 1.0,
				ExporterURL:  "https://localhost:8080",
			},
			errorAssertion: require.NoError,
			expectedCfg: Config{
				Service:      "test",
				ExporterURL:  "https://localhost:8080",
				SamplingRate: 1.0,
				DialTimeout:  DefaultExporterDialTimeout,
			},
			expectedURL: &url.URL{
				Scheme: "https",
				Host:   "localhost:8080",
			},
		},
		{
			name: "file exporter",
			cfg: Config{
				Service:      "test",
				SamplingRate: 1.0,
				ExporterURL:  "file:///var/lib/teleport",
				DialTimeout:  time.Millisecond,
			},
			errorAssertion: require.NoError,
			expectedCfg: Config{
				Service:      "test",
				ExporterURL:  "file:///var/lib/teleport",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expectedURL: &url.URL{
				Scheme: "file",
				Host:   "/var/lib/teleport",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.CheckAndSetDefaults()
			tt.errorAssertion(t, err)
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(tt.expectedCfg, tt.cfg,
				cmpopts.IgnoreUnexported(Config{}),
				cmpopts.IgnoreFields(Config{}, "Logger"),
			))
			require.NotNil(t, tt.cfg.Logger)
		})
	}
}

func TestConfig_Endpoint(t *testing.T) {
	cases := []struct {
		name     string
		cfg      Config
		expected string
	}{
		{
			name: "with http scheme",
			cfg: Config{
				Service:     "test",
				ExporterURL: "http://localhost:8080",
			},
			expected: "localhost:8080",
		},
		{
			name: "with https scheme",
			cfg: Config{
				Service:     "test",
				ExporterURL: "https://localhost:8080/custom",
			},
			expected: "localhost:8080/custom",
		},
		{
			name: "with grpc scheme",
			cfg: Config{
				Service:      "test",
				ExporterURL:  "grpc://collector.opentelemetry.svc:4317",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expected: "collector.opentelemetry.svc:4317",
		},
		{
			name: "without a scheme",
			cfg: Config{
				Service:      "test",
				ExporterURL:  "collector.opentelemetry.svc:4317",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expected: "collector.opentelemetry.svc:4317",
		},
		{
			name: "file exporter",
			cfg: Config{
				Service:      "test",
				ExporterURL:  "file:///var/lib/teleport",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expected: "/var/lib/teleport",
		},
		{
			name: "file exporter with limit",
			cfg: Config{
				Service:      "test",
				ExporterURL:  "file:///var/lib/teleport?limit=200",
				SamplingRate: 1.0,
				DialTimeout:  time.Millisecond,
			},
			expected: "/var/lib/teleport",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.cfg.CheckAndSetDefaults())
			require.Equal(t, tt.expected, tt.cfg.Endpoint())
		})
	}
}

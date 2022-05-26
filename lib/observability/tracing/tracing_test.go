// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracing

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func generateTLSCertificate() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

type collector struct {
	grpcLn     net.Listener
	httpLn     net.Listener
	grpcServer *grpc.Server
	httpServer *http.Server

	coltracepb.TraceServiceServer
	spans []*otlp.ScopeSpans
}

type collectorConfig struct {
	WithTLS bool
}

func newCollector(cfg collectorConfig) (*collector, error) {
	grpcLn, err := net.Listen("tcp4", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpLn, err := net.Listen("tcp4", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tlsConfig *tls.Config
	creds := insecure.NewCredentials()
	if cfg.WithTLS {
		tlsCertificate, err := generateTLSCertificate()
		if err != nil {
			return nil, err
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCertificate},
		}
		creds = credentials.NewTLS(tlsConfig)
	}

	c := &collector{
		grpcLn:     grpcLn,
		httpLn:     httpLn,
		grpcServer: grpc.NewServer(grpc.Creds(creds)),
	}

	c.httpServer = &http.Server{Handler: c, TLSConfig: tlsConfig}

	coltracepb.RegisterTraceServiceServer(c.grpcServer, c)

	return c, nil
}

func (c *collector) ClientTLSConfig() *tls.Config {
	if c.httpServer.TLSConfig == nil {
		return nil
	}

	return &tls.Config{
		InsecureSkipVerify: true,
	}
}

func (c collector) GRPCAddr() string {
	return "grpc://" + c.grpcLn.Addr().String()
}

func (c collector) HTTPAddr() string {
	return "http://" + c.httpLn.Addr().String()
}

func (c collector) HTTPSAddr() string {
	return "https://" + c.httpLn.Addr().String()
}

func (c collector) Start() error {
	var g errgroup.Group

	g.Go(func() error {
		if c.httpServer.TLSConfig != nil {
			return c.httpServer.ServeTLS(c.httpLn, "", "")
		}
		return c.httpServer.Serve(c.httpLn)
	})

	g.Go(func() error {
		return c.grpcServer.Serve(c.grpcLn)
	})

	return g.Wait()
}

func (c collector) Shutdown(ctx context.Context) error {
	c.grpcServer.Stop()
	return c.httpServer.Shutdown(ctx)
}

func (c *collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v1/traces" {
		response := coltracepb.ExportTraceServiceResponse{}
		rawResponse, err := proto.Marshal(&response)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		rawRequest, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		req := &coltracepb.ExportTraceServiceRequest{}
		if err := proto.Unmarshal(rawRequest, req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		for _, span := range req.ResourceSpans {
			c.spans = append(c.spans, span.ScopeSpans...)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(rawResponse)
	}
}

func (c *collector) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	for _, span := range req.ResourceSpans {
		c.spans = append(c.spans, span.ScopeSpans...)
	}

	return &coltracepb.ExportTraceServiceResponse{}, nil
}

func TestNewExporter(t *testing.T) {
	t.Parallel()
	c, err := newCollector(collectorConfig{
		WithTLS: false,
	})
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
			name: "connection timeout",
			config: Config{
				Service:     "test",
				ExporterURL: "localhost:123",
				DialTimeout: time.Millisecond,
			},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err, i...)
				require.True(t, trace.IsConnectionProblem(err), i...)
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
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := NewExporter(context.Background(), tt.config)
			tt.errAssertion(t, err)
			tt.exporterAssertion(t, exporter)
		})
	}
}

func TestTraceProvider(t *testing.T) {
	t.Parallel()
	const spansCreated = 4
	cases := []struct {
		name              string
		config            func(c *collector) Config
		errAssertion      require.ErrorAssertionFunc
		providerAssertion require.ValueAssertionFunc
		collectedLen      int
		withTLS           bool
	}{
		{
			name: "not sampling prevents exporting",
			config: func(c *collector) Config {
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
			config: func(c *collector) Config {
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
			withTLS:           true,
		},
		{
			name: "spans exported with gRPC",
			config: func(c *collector) Config {
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
			config: func(c *collector) Config {
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
			config: func(c *collector) Config {
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
			withTLS:           true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			collector, err := newCollector(collectorConfig{
				WithTLS: tt.withTLS,
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
			provider, err := NewTraceProvider(ctx, tt.config(collector))
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
			require.LessOrEqual(t, len(collector.spans), tt.collectedLen)
			require.GreaterOrEqual(t, len(collector.spans), 0)
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
				cmpopts.IgnoreInterfaces(struct{ logrus.FieldLogger }{})),
			)
			require.NotNil(t, tt.cfg.Logger)
		})
	}
}

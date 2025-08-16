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

package servicecfg

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"
)

// MetricsConfig specifies configuration for the metrics service
type MetricsConfig struct {
	// Enabled turns the metrics service role on or off for this process
	Enabled bool

	// ListenAddr is the address to listen on for incoming metrics requests.
	// Optional.
	ListenAddr *utils.NetAddr

	// MTLS turns mTLS on the metrics service on or off
	MTLS bool

	// KeyPairs are the key and certificate pairs that the metrics service will
	// use for mTLS.
	// Used in conjunction with MTLS = true
	KeyPairs []KeyPairPath

	// CACerts are prometheus ca certs
	// use for mTLS.
	// Used in conjunction with MTLS = true
	CACerts []string

	// GRPCServerLatency enables histogram metrics for each grpc endpoint on the auth server
	GRPCServerLatency bool

	// GRPCServerLatency enables histogram metrics for each grpc endpoint on the auth server
	GRPCClientLatency bool
}

// TracingConfig specifies the configuration for the tracing service
type TracingConfig struct {
	// Enabled turns the tracing service role on or off for this process.
	Enabled bool

	// ExporterURL is the OTLP exporter URL to send spans to.
	ExporterURL string

	// KeyPairs are the paths for key and certificate pairs that the tracing
	// service will use for outbound TLS connections.
	KeyPairs []KeyPairPath

	// CACerts are the paths to the CA certs used to validate the collector.
	CACerts []string

	// SamplingRate is the sampling rate for the exporter.
	// 1.0 will record and export all spans and 0.0 won't record any spans.
	SamplingRate float64
}

// Config generates a tracing.Config that is populated from the values
// provided to the tracing_service
func (t TracingConfig) Config(attrs ...attribute.KeyValue) (*tracing.Config, error) {
	traceConf := &tracing.Config{
		Service:      teleport.ComponentTeleport,
		Attributes:   attrs,
		ExporterURL:  t.ExporterURL,
		SamplingRate: t.SamplingRate,
	}

	tlsConfig := &tls.Config{}
	// if a custom CA is specified, use a custom cert pool
	if len(t.CACerts) > 0 {
		pool := x509.NewCertPool()
		for _, caCertPath := range t.CACerts {
			caCert, err := os.ReadFile(caCertPath)
			if err != nil {
				return nil, trace.Wrap(err, "failed to read tracing CA certificate %+v", caCertPath)
			}

			if !pool.AppendCertsFromPEM(caCert) {
				return nil, trace.BadParameter("failed to parse tracing CA certificate: %+v", caCertPath)
			}
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.RootCAs = pool
	}

	// add any custom certificates for mTLS
	if len(t.KeyPairs) > 0 {
		for _, pair := range t.KeyPairs {
			certificate, err := tls.LoadX509KeyPair(pair.Certificate, pair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err, "failed to read keypair: %+v", err)
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
		}
	}

	if len(t.CACerts) > 0 || len(t.KeyPairs) > 0 {
		traceConf.TLSConfig = tlsConfig
	}
	return traceConf, nil
}

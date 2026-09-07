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
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/url"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"
)

const maxSamplingRatePerMillion = 1_000_000

// TracingConfig controls the export of distributed traces. It mirrors the
// `tracing_service` section of the Teleport configuration.
type TracingConfig struct {
	Enabled bool `yaml:"enabled"`
	// ExporterURL is the OTLP exporter to send spans to. Supports grpc://,
	// http(s):// and file:// schemes.
	ExporterURL string `yaml:"exporter_url,omitempty"`
	// SamplingRatePerMillion is how many spans out of every million are
	// sampled. Unset means all spans are sampled.
	SamplingRatePerMillion *int `yaml:"sampling_rate_per_million,omitempty"`
	// CACerts are paths to PEM encoded CA certificates used to verify the
	// exporter.
	CACerts []string `yaml:"ca_certs,omitempty"`
	// KeyPairs are client certificates presented to the exporter for mTLS.
	KeyPairs []KeyPair `yaml:"keypairs,omitempty"`
}

// KeyPair is a pair of paths to a PEM encoded private key and certificate.
type KeyPair struct {
	PrivateKey  string `yaml:"key_file"`
	Certificate string `yaml:"cert_file"`
}

func (c *TracingConfig) CheckAndSetDefaults() error {
	if !c.Enabled {
		return nil
	}
	if c.ExporterURL == "" {
		return trace.BadParameter("exporter_url: must be specified when tracing is enabled")
	}
	scheme := c.exporterScheme()
	switch scheme {
	case "grpc", "https":
	case "http", "file":
		if len(c.CACerts) > 0 {
			return trace.BadParameter("ca_certs: cannot be used with the %s scheme", scheme)
		}
		if len(c.KeyPairs) > 0 {
			return trace.BadParameter("keypairs: cannot be used with the %s scheme", scheme)
		}
	default:
		return trace.BadParameter("exporter_url: must use one of the grpc, http, https or file schemes")
	}
	if r := c.SamplingRatePerMillion; r != nil && (*r < 0 || *r > maxSamplingRatePerMillion) {
		return trace.BadParameter("sampling_rate_per_million: must be between 0 and %d", maxSamplingRatePerMillion)
	}
	for _, p := range c.KeyPairs {
		if !utils.FileExists(p.PrivateKey) {
			return trace.NotFound("keypairs: private key does not exist: %s", p.PrivateKey)
		}
		if !utils.FileExists(p.Certificate) {
			return trace.NotFound("keypairs: certificate does not exist: %s", p.Certificate)
		}
	}
	for _, caCert := range c.CACerts {
		if !utils.FileExists(caCert) {
			return trace.NotFound("ca_certs: file does not exist: %s", caCert)
		}
	}
	return nil
}

// TraceConfig builds the configuration for the tracing provider, loading any
// TLS material from disk.
func (c *TracingConfig) TraceConfig() (*tracing.Config, error) {
	cfg := &tracing.Config{
		Service:      teleport.ComponentTBot,
		ExporterURL:  c.ExporterURL,
		SamplingRate: 1.0,
	}
	if c.SamplingRatePerMillion != nil {
		cfg.SamplingRate = float64(*c.SamplingRatePerMillion) / maxSamplingRatePerMillion
	}
	// Unlike the Teleport tracing_service, an https exporter uses TLS even when
	// no CA or keypair is configured, rather than silently downgrading to http.
	if c.exporterScheme() != "https" && len(c.CACerts) == 0 && len(c.KeyPairs) == 0 {
		return cfg, nil
	}

	tlsConfig := &tls.Config{}
	if len(c.CACerts) > 0 {
		pool := x509.NewCertPool()
		for _, path := range c.CACerts {
			pem, err := os.ReadFile(path)
			if err != nil {
				return nil, trace.Wrap(err, "reading tracing CA certificate %q", path)
			}
			if !pool.AppendCertsFromPEM(pem) {
				return nil, trace.BadParameter("parsing tracing CA certificate %q", path)
			}
		}
		tlsConfig.RootCAs = pool
	}
	for _, pair := range c.KeyPairs {
		cert, err := tls.LoadX509KeyPair(pair.Certificate, pair.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err, "loading tracing keypair %q", pair.Certificate)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}
	cfg.TLSConfig = tlsConfig
	return cfg, nil
}

// exporterScheme mirrors how lib/observability/tracing interprets the URL: a
// bare host:port is treated as grpc.
func (c *TracingConfig) exporterScheme() string {
	if h, _, err := net.SplitHostPort(c.ExporterURL); err == nil && h != "file" {
		return "grpc"
	}
	u, err := url.Parse(c.ExporterURL)
	if err != nil {
		return ""
	}
	return u.Scheme
}

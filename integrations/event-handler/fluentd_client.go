/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	tlib "github.com/gravitational/teleport/integrations/lib"
)

const (
	// httpTimeout is the maximum HTTP timeout
	httpTimeout = 30 * time.Second
)

// FluentdClient represents Fluentd client
type FluentdClient struct {
	// client HTTP client to send requests
	client *http.Client
}

// NewFluentdClient creates new FluentdClient
func NewFluentdClient(c *FluentdConfig) (*FluentdClient, error) {
	var certs []tls.Certificate
	if c.FluentdCert != "" && c.FluentdKey != "" {
		cert, err := tls.LoadX509KeyPair(c.FluentdCert, c.FluentdKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs = append(certs, cert)
	} else if c.FluentdCert != "" || c.FluentdKey != "" {
		return nil, trace.BadParameter("both fluentd_cert and fluentd_key should be specified")
	}

	ca, err := getCertPool(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      ca,
				Certificates: certs,
			},
		},
		Timeout: httpTimeout,
	}

	return &FluentdClient{client: client}, nil
}

// getCertPool reads CA certificate and returns CA cert pool if passed
func getCertPool(c *FluentdConfig) (*x509.CertPool, error) {
	if c.FluentdCA == "" {
		return nil, nil
	}

	caCert, err := os.ReadFile(c.FluentdCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return caCertPool, nil
}

// Send sends event to fluentd
func (f *FluentdClient) Send(ctx context.Context, url string, b []byte) error {
	log.WithField("json", string(b)).Debug("JSON to send")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return trace.Wrap(err)
	}
	req.Header.Add("Content-Type", "application/json")

	r, err := f.client.Do(req)
	if err != nil {
		// err returned by client.Do() would never have status canceled
		if tlib.IsCanceled(ctx.Err()) {
			return trace.Wrap(ctx.Err())
		}

		return trace.Wrap(err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return trace.Errorf("Failed to send event to fluentd (HTTP %v)", r.StatusCode)
	}

	return nil
}

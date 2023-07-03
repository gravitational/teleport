/*
Copyright 2023 Gravitational, Inc.

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

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestALPNDialer_getTLSConfig(t *testing.T) {
	t.Parallel()
	cas := x509.NewCertPool()

	tests := []struct {
		name          string
		input         ALPNDialerConfig
		wantTLSConfig *tls.Config
		wantError     bool
	}{
		{
			name:      "missing tls config",
			input:     ALPNDialerConfig{},
			wantError: true,
		},
		{
			name: "no update",
			input: ALPNDialerConfig{
				TLSConfig: &tls.Config{
					ServerName: "example.com",
				},
			},
			wantTLSConfig: &tls.Config{
				ServerName: "example.com",
			},
		},
		{
			name: "no update when upgrade required",
			input: ALPNDialerConfig{
				TLSConfig: &tls.Config{
					ServerName: "example.com",
					RootCAs:    cas,
				},
				ALPNConnUpgradeRequired: true,
			},
			wantTLSConfig: &tls.Config{
				ServerName: "example.com",
				RootCAs:    cas,
			},
		},
		{
			name: "name updated",
			input: ALPNDialerConfig{
				TLSConfig: &tls.Config{},
			},
			wantTLSConfig: &tls.Config{
				ServerName: "example.com",
			},
		},
		{
			name: "get cas failed",
			input: ALPNDialerConfig{
				TLSConfig:               &tls.Config{},
				ALPNConnUpgradeRequired: true,
				GetClusterCAs: func(_ context.Context) (*x509.CertPool, error) {
					return nil, trace.AccessDenied("fail it")
				},
			},
			wantError: true,
		},
		{
			name: "cas updated",
			input: ALPNDialerConfig{
				TLSConfig:               &tls.Config{},
				ALPNConnUpgradeRequired: true,
				GetClusterCAs:           ClusterCAsFromCertPool(cas),
			},
			wantTLSConfig: &tls.Config{
				ServerName: "example.com",
				RootCAs:    cas,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialer := NewALPNDialer(test.input).(*ALPNDialer)
			tlsConfig, err := dialer.getTLSConfig(context.Background(), "example.com:443")
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantTLSConfig, tlsConfig)
			}
		})
	}
}

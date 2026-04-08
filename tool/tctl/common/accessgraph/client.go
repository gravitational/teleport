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

package accessgraph

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	accessgraph "github.com/gravitational/access-graph/api/client"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/trace"
)

type AccessGraphError struct {
	StatusCode int
	Message    string
}

type AccessGraphResponse interface {
	StatusCode() int
	GetBody() []byte
}

func (e *AccessGraphError) Error() string {
	return fmt.Sprintf("API request failed with status %d: %s", e.StatusCode, e.Message)
}

func doRequest[T AccessGraphResponse](resp T, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, fmt.Errorf("request failed: %w", err)
	}
	body := resp.GetBody()
	if err := checkResponse(resp.StatusCode(), body); err != nil {
		var zero T
		return zero, err
	}
	return resp, nil
}

func checkResponse(statusCode int, body []byte) error {
	if statusCode >= 400 {
		var badReq accessgraph.BadRequest
		if err := json.Unmarshal(body, &badReq); err != nil {
			return fmt.Errorf("request failed with status %d", statusCode)
		}
		return &AccessGraphError{
			StatusCode: statusCode,
			Message:    badReq.Message,
		}
	}
	return nil
}

func newAccessGraphHTTPClient(ctx context.Context, client *authclient.Client) (*accessgraph.ClientWithResponses, error) {
	pingResp, err := client.Ping(ctx)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	if pingResp.GetProxyPublicAddr() == "" {
		return nil, trace.NotFound("proxy public address is not configured")
	}

	currentUser, err := client.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	existingTLSConfig := client.HTTPClient.TLSConfig()
	if existingTLSConfig == nil {
		return nil, trace.BadParameter("missing auth client TLS config")
	}

	signer, err := existingTLSSigner(existingTLSConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKeyPEM, certs, err := generateAccessGraphUserCerts(ctx, client, signer, currentUser.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCert, err := tls.X509KeyPair(certs.TLS, privateKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyHost, _, err := webclient.ParseHostPort(pingResp.GetProxyPublicAddr())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	baseTLSConfig, err := newAccessGraphTLSConfig(proxyHost, clientCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: baseTLSConfig,
			Proxy:           http.ProxyFromEnvironment,
		},
		Timeout: 30 * time.Second,
	}

	accessGraphClient, err := accessgraph.NewClientWithResponses(
		pingResp.GetProxyPublicAddr(),
		accessgraph.WithHTTPClient(&httpClient),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessGraphClient, nil
}

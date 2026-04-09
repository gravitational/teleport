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
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	accessgraph "github.com/gravitational/access-graph/api/client"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	libhwk "github.com/gravitational/teleport/lib/hardwarekey"
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

type teleportErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
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
		if err := json.Unmarshal(body, &badReq); err == nil && badReq.Message != "" {
			return &AccessGraphError{
				StatusCode: statusCode,
				Message:    badReq.Message,
			}
		}

		var teleportErr teleportErrorResponse
		if err := json.Unmarshal(body, &teleportErr); err == nil && teleportErr.Error.Message != "" {
			return &AccessGraphError{
				StatusCode: statusCode,
				Message:    teleportErr.Error.Message,
			}
		}

		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("request failed with status %d", statusCode)
		}

		return &AccessGraphError{
			StatusCode: statusCode,
			Message:    message,
		}
	}
	return nil
}

func (c *AccessGraphCommand) generateAccessGraphClient(ctx context.Context, proxyAddr string) (*accessgraph.ClientWithResponses, error) {
	if len(c.ccf.AuthServerAddr) != 0 {
		proxyAddr = c.ccf.AuthServerAddr[0]
	}

	hwks := libhwk.NewService(ctx, nil /* prompt */)
	clientStore := client.NewFSClientStore(c.config.TeleportHome, client.WithHardwareKeyService(hwks))
	if c.ccf.IdentityFilePath != "" {
		clientStore = client.NewMemClientStore(client.WithHardwareKeyService(hwks))
		if err := identityfile.LoadIdentityFileIntoClientStore(clientStore, c.ccf.IdentityFilePath, proxyAddr, ""); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	profile, err := clientStore.ReadProfileStatus(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.IsExpired(time.Now()) {
		return nil, trace.BadParameter("your credentials have expired, please log in using `tsh login`")
	}

	idx := client.KeyRingIndex{ProxyHost: profile.Name, Username: profile.Username}
	keyRing, err := clientStore.GetKeyRing(idx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := newAccessGraphHTTPClient(proxyAddr, keyRing)
	return client, nil
}

func newAccessGraphHTTPClient(proxyAddr string, keyRing *client.KeyRing) (*accessgraph.ClientWithResponses, error) {
	if keyRing == nil {
		return nil, trace.BadParameter("missing key ring")
	}

	baseTLSConfig, err := keyRing.AccessGraphClientTLSConfig(nil, nil)
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

	slog.Debug("Initialized Access Graph HTTP client with TLS config from keyring", "proxyAddr", proxyAddr)

	accessGraphClient, err := accessgraph.NewClientWithResponses(
		fmt.Sprintf("https://%s/v1/enterprise/accessgraph/", proxyAddr),
		accessgraph.WithHTTPClient(&httpClient),
	)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessGraphClient, nil
}

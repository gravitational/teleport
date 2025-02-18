/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// Package podman implements bindings for a small subset of Podman's REST API.
//
// We use this instead of importing github.com/containers/podman/v5/pkg/bindings
// because we only use a couple of endpoints, so it's not worth taking on its
// transitive dependencies.
package podman

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// APIVersion is the version of the Podman REST API we support.
//
// Note: Podman doesn't really seem to check or enforce the version segment
// of the URL path, so setting it doesn't have much of an effect.
//
// https://github.com/containers/podman/blob/62cde17193e1469de15995ba78bd909fd07dffc6/pkg/api/server/handler_api.go#L69-L74
const APIVersion = "v4.0.0"

// Client for Podman's HTTP REST API.
type Client struct{ httpClient *http.Client }

// NewClient creates a Client for the API service at the given address.
//
// Note: addr must be in the form `unix://path/to/socket`, we do not currently
// support connecting to the Podman service over TCP because it's generally
// insecure.
func NewClient(addr string) (*Client, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, trace.Wrap(err, "invalid address: %q", addr)
	}
	if u.Scheme != "unix" {
		return nil, trace.BadParameter("only unix domain sockets are supported")
	}
	return &Client{
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(
				&http.Transport{
					DialContext: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
						return (&net.Dialer{}).DialContext(ctx, "unix", u.Path)
					},
				},
				otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
					return fmt.Sprintf("Podman API %s %s", r.Method, r.URL.Path)
				}),
			),
		},
	}, nil
}

// InspectContainer reads information about the container with the given ID.
//
// https://docs.podman.io/en/latest/_static/api.html#tag/containers/operation/ContainerInspectLibpod
func (c *Client) InspectContainer(ctx context.Context, id string) (*Container, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("containers/%s", id))
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	var cntr Container
	if err := json.NewDecoder(rsp.Body).Decode(&cntr); err != nil {
		return nil, trace.Wrap(err, "failed to decode container response")
	}
	return &cntr, nil
}

// InspectPod reads information about the pod with the given ID.
//
// https://docs.podman.io/en/latest/_static/api.html#tag/pods/operation/PodInspectLibpod
func (c *Client) InspectPod(ctx context.Context, id string) (*Pod, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("pods/%s", id))
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	var pod Pod
	if err := json.NewDecoder(rsp.Body).Decode(&pod); err != nil {
		return nil, trace.Wrap(err, "failed to decode pod response")
	}
	return &pod, nil
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	url := fmt.Sprintf("http://d/%s/libpod/%s/json", APIVersion, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	rsp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode == http.StatusOK {
		return rsp, nil
	}

	if err := rsp.Body.Close(); err != nil {
		return nil, trace.Wrap(err, "failed to close response body")
	}
	switch rsp.StatusCode {
	case http.StatusNotFound:
		return nil, trace.NotFound("not found")
	default:
		return nil, trace.Errorf("unexpected response from Podman service (status: %d)", rsp.StatusCode)
	}
}

// Container details.
type Container struct {
	// ID is the full 64-character identifier of the container.
	ID string `json:"Id"`

	// Name is the user-given name of the container.
	Name string

	// Config contains the initial configuration of the container.
	Config ContainerConfig
}

// ContainerConfig is the initial configuration of a container.
type ContainerConfig struct {
	// Image name (e.g. nginx:latest)
	Image string

	// Labels given to the container.
	Labels map[string]string
}

// Pod details.
type Pod struct {
	// ID is the full 64-character identifier of the pod.
	ID string `json:"Id"`

	// Name is the user-given name of the pod.
	Name string

	// Labels given to the pod.
	Labels map[string]string
}

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

package gcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"cloud.google.com/go/compute/metadata"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// contextRoundTripper is a http.RoundTripper that adds a context.Context to
// requests.
type contextRoundTripper struct {
	ctx       context.Context
	transport http.RoundTripper
}

func (rt contextRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.transport.RoundTrip(req.WithContext(rt.ctx))
	return resp, trace.Wrap(err)
}

// getMetadataClient gets an instance metadata client that will use the
// provided context.
func getMetadataClient(ctx context.Context) (*metadata.Client, error) {
	transport, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return metadata.NewClient(&http.Client{
		Transport: contextRoundTripper{
			ctx:       ctx,
			transport: transport,
		},
	}), nil
}

func convertMetadataError(err error) error {
	var metadataErr *metadata.Error
	if errors.As(err, &metadataErr) {
		return trace.ReadError(metadataErr.Code, []byte(metadataErr.Message))
	}
	return err
}

// get gets GCP instance metadata from an arbitrary path.
func getMetadata(ctx context.Context, suffix string) (string, error) {
	client, err := getMetadataClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	resp, err := client.GetWithContext(ctx, suffix)
	return resp, trace.Wrap(convertMetadataError(err))
}

// GetIDToken gets an ID token from GCP instance metadata.
func GetIDToken(ctx context.Context) (string, error) {
	audience := "teleport.cluster.local"
	resp, err := getMetadata(ctx, fmt.Sprintf(
		"instance/service-accounts/default/identity?audience=%s&format=full&licenses=FALSE",
		url.QueryEscape(audience),
	))
	return resp, trace.Wrap(err)
}

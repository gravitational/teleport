// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package top

import (
	"context"
	"errors"
	"net/url"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib"
)

type ClientConfig struct {
	addr string
	opts []roundtrip.ClientParam
}

func tryNewClient(ctx context.Context, cfgs ...ClientConfig) (*roundtrip.Client, error) {
	var errs []error
	for _, config := range cfgs {
		client, err := roundtrip.NewClient(config.addr, "", config.opts...)
		if err != nil {
			errs = append(errs, trace.Wrap(err, "failed create client for %v", config.addr))
			continue
		}

		if _, err = httplib.ConvertResponse(client.Get(ctx, client.Endpoint("metrics"), url.Values{})); err != nil {
			errs = append(errs, trace.Wrap(err, "failed to fetch metrics from addr %v", config.addr))
			continue
		}

		return client, nil
	}

	return nil, errors.Join(errs...)
}

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

package services

import (
	"github.com/gravitational/trace"

	accessgraphsecretspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
)

// MarshalAccessGraphAuthorizedKey marshals a [accessgraphsecretspb.AuthorizedKey] resource to JSON.
func MarshalAccessGraphAuthorizedKey(in *accessgraphsecretspb.AuthorizedKey, opts ...MarshalOption) ([]byte, error) {
	if err := accessgraph.ValidateAuthorizedKey(in); err != nil {
		return nil, trace.Wrap(err)
	}

	return MarshalProtoResource(in, opts...)
}

// UnmarshalAccessGraphAuthorizedKey unmarshals a [accessgraphsecretspb.AuthorizedKey] resource from JSON.
func UnmarshalAccessGraphAuthorizedKey(data []byte, opts ...MarshalOption) (*accessgraphsecretspb.AuthorizedKey, error) {
	out, err := UnmarshalProtoResource[*accessgraphsecretspb.AuthorizedKey](data, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := accessgraph.ValidateAuthorizedKey(out); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// MarshalAccessGraphPrivateKey marshals a [accessgraphsecretspb.PrivateKey] resource to JSON.
func MarshalAccessGraphPrivateKey(in *accessgraphsecretspb.PrivateKey, opts ...MarshalOption) ([]byte, error) {
	if err := accessgraph.ValidatePrivateKey(in); err != nil {
		return nil, trace.Wrap(err)
	}

	return MarshalProtoResource(in, opts...)
}

// UnmarshalAccessGraphPrivateKey unmarshals a [accessgraphsecretspb.PrivateKey] resource from JSON.
func UnmarshalAccessGraphPrivateKey(data []byte, opts ...MarshalOption) (*accessgraphsecretspb.PrivateKey, error) {
	out, err := UnmarshalProtoResource[*accessgraphsecretspb.PrivateKey](data, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := accessgraph.ValidatePrivateKey(out); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

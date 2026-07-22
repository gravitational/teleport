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

package pinning

import (
	"encoding/base64"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
)

// Encode encodes the given Scope Pin into its compact encoding. This is the official encoding intended to be used in certificate extensions
// and other space-constrained contexts. This encoding achieves compatness by a combination of protobuf binary encoding and unpadded urlsafe base64.
// Contexts that are not space constrained may prefer to use protojson encoding instead for easier debugging.
func Encode(pin *scopesv1.Pin) (string, error) {
	penc, err := proto.Marshal(pin)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if len(penc) == 0 {
		// protobuf considers an empty message to be valid and will encode it as an empty byte slice, however for our purposes
		// scope pins should never be empty and having encode return an empty string may lead to confusion.
		return "", trace.BadParameter("cannot encode empty scope pin")
	}

	return base64.RawURLEncoding.EncodeToString(penc), nil
}

// Decode decodes the given compactly encoded Scope Pin. The input must have been encoded using the [Encode] function. As
// an additional fallback, the Decode function will also attempt to decode the input as the old JSON encoded format. We
// don't *need* decode to continue to handle json input as the scopes feature was still highly unstable at the time we
// broke compatibility with the json encoding, but some confusing errors are avoided by having this fallback, so it may
// smooth some things over for people who were experimenting with scopes during its infancy.
func Decode(encoded string) (*scopesv1.Pin, error) {
	if encoded == "" {
		// in theory a an empty string _would_ be a valid base64 encoding of an empty byte slice, which protobuf would decode
		// as an empty message. However, for our purposes scope pins should never be empty and having decode accept an empty
		// string may lead to confusion.
		return nil, trace.BadParameter("cannot decode empty scope pin encoding")
	}
	penc, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		if strings.HasPrefix(encoded, "{") {
			// fallback to json decoding for compatibility with old format. this fallback is not mandated by compatibility
			// and may be removed at any time without warning.
			var pin scopesv1.Pin
			if jsonErr := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(encoded), &pin); jsonErr != nil {
				return nil, trace.Wrap(err)
			}
			return &pin, nil
		}
		return nil, trace.Wrap(err)
	}
	var pin scopesv1.Pin
	if err := proto.Unmarshal(penc, &pin); err != nil {
		return nil, trace.Wrap(err)
	}

	return &pin, nil
}

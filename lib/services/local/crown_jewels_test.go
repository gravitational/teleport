/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func TestMarshalCrownJewelRoundTrip(t *testing.T) {
	t.Parallel()

	spec := &crownjewelv1.CrownJewelSpec{}
	obj := &crownjewelv1.CrownJewel{
		Metadata: &headerv1.Metadata{
			Name: "dummy-crown-jewel",
		},
		Spec: spec,
	}

	out, err := MarshalCrownJewel(obj)
	require.NoError(t, err)
	newObj, err := UnmarshalCrownJewel(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}

//TODO(jaku): add more tests

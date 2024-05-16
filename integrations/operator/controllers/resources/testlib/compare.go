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

package testlib

import (
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

// MaxTimeDiff is used when writing tests comparing times. Timestamps suffer
// from being serialized/deserialized with different precisions, the final value
// might be a bit different from the initial one. If the difference is less than
// MaxTimeDiff, we consider the values equal.
const MaxTimeDiff = 2 * time.Millisecond

var defaultCompareOpts = []cmp.Option{
	// Ensures that a nil map equates initialized but empty map (depending on how the resource is sent over the
	// wire, some local nil fields might come back from Teleport initialized but empty, and vice-versa.
	cmpopts.EquateEmpty(),
	// Fixes time precision issues that might be caused by serialization/deserialization
	cmpopts.EquateApproxTime(MaxTimeDiff),
	// New resource headers
	cmpopts.IgnoreFields(header.Metadata{}, "Labels", "Revision"),
	cmpopts.IgnoreFields(header.ResourceHeader{}, "Kind"),
	// Legacy gogoproto resource headers
	cmpopts.IgnoreFields(types.Metadata{}, "Labels", "Revision", "Namespace"),
	cmpopts.IgnoreFields(types.ResourceHeader{}, "Kind"),
}

// CompareOptions builds comparison options by returning a slice of both default comparison options and optional
// custom ones for the test/resource.
func CompareOptions(customOpts ...cmp.Option) []cmp.Option {
	return append(defaultCompareOpts, customOpts...)
}

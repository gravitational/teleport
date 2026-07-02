// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

// Package foos contains general utilities for interacting with Foos
package foos

import (
	"github.com/gravitational/trace"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

const (
	// Kind
	Kind = "foo"
)

// StrongValidate strongly validates a foo, must be called before writing it to storage.
func StrongValidate(foo *foov1.Foo) error {
	if scope := foo.GetScope(); scope != "" {
		if err := scopes.StrongValidate(scope); err != nil {
			return trace.Wrap(err)
		}
	}
	if foo.GetKind() != Kind {
		return trace.BadParameter("kind must be %s", Kind)
	}
	if foo.GetVersion() != types.V1 {
		return trace.BadParameter("version must be %s", types.V1)
	}
	if foo.GetSpec().GetValue() == "" {
		return trace.BadParameter("spec.value must be non-empty")
	}
	return nil
}

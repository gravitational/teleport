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

package parser

import (
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func validateRequestShapes(storage spec.StorageConfig, reqs map[string]protoreflect.MessageDescriptor) error {
	switch storage.Pattern {
	case spec.StoragePatternStandard:
		if err := requireField(reqs["get"], "name"); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["delete"], "name"); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["list"], "page_size"); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["list"], "page_token"); err != nil {
			return trace.Wrap(err)
		}
	case spec.StoragePatternSingleton:
		if reqs["list"] != nil {
			return trace.BadParameter("singleton storage does not support List operations")
		}
		if hasField(reqs["get"], "name") {
			return trace.BadParameter("singleton Get request must not contain name field")
		}
		if hasField(reqs["delete"], "name") {
			return trace.BadParameter("singleton Delete request must not contain name field")
		}
	case spec.StoragePatternScoped:
		if err := requireField(reqs["get"], storage.ScopeBy); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["get"], "name"); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["delete"], storage.ScopeBy); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["delete"], "name"); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["list"], storage.ScopeBy); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["list"], "page_size"); err != nil {
			return trace.Wrap(err)
		}
		if err := requireField(reqs["list"], "page_token"); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported storage pattern: %q", storage.Pattern)
	}

	return nil
}

func requireField(msg protoreflect.MessageDescriptor, field string) error {
	if msg == nil {
		return nil
	}
	if !hasField(msg, field) {
		return trace.BadParameter("%s is missing required field %q", msg.FullName(), field)
	}
	return nil
}

func hasField(msg protoreflect.MessageDescriptor, field string) bool {
	if msg == nil {
		return false
	}
	return msg.Fields().ByName(protoreflect.Name(field)) != nil
}

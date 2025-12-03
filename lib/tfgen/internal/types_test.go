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

package internal_test

import (
	"time"

	"github.com/gravitational/teleport/lib/tfgen/internal"
)

func attribute(name string, value *internal.Value) internal.Attribute {
	return internal.Attribute{
		Name:  name,
		Value: value,
	}
}

func messageVal(attributes ...internal.Attribute) *internal.Value {
	return &internal.Value{
		Type: internal.TypeMessage,
		Value: &internal.Message{
			Attributes: attributes,
		},
	}
}

func stringVal(s string) *internal.Value {
	return &internal.Value{
		Type:  internal.TypeString,
		Value: s,
	}
}

func timestampVal(t time.Time) *internal.Value {
	return &internal.Value{
		Type:  internal.TypeTimestamp,
		Value: t,
	}
}

func durationVal(t time.Duration) *internal.Value {
	return &internal.Value{
		Type:  internal.TypeDuration,
		Value: t,
	}
}

func mapVal(elems map[any]*internal.Value) *internal.Value {
	if elems == nil {
		elems = make(map[any]*internal.Value)
	}
	return &internal.Value{
		Type: internal.TypeMap,
		Value: &internal.MapValue{
			Elems: elems,
		},
	}
}

func listVal(elems ...*internal.Value) *internal.Value {
	if elems == nil {
		elems = []*internal.Value{}
	}
	return &internal.Value{
		Type: internal.TypeList,
		Value: &internal.ListValue{
			Elems: elems,
		},
	}
}

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

package transform

import "github.com/gravitational/teleport/lib/tfgen/internal"

// BotTraits transforms the standard representation of a slice of
// machineidv1.Trait values into the map[string][]string form we
// expect in the Terraform provider.
//
// Before:
//
//	[{name = "logins", values = ["root", "ubuntu"]}]
//
// After:
//
//	{logins = ["root", "ubuntu"]}
//
// Example usage:
//
//	tfgen.Generate(bot, tfgen.WithFieldTransform("spec.traits", transform.BotTraits))
func BotTraits(value *internal.Value) (*internal.Value, error) {
	traitsMap := make(map[any]*internal.Value)

	for _, elem := range value.List().Elems {
		message := elem.Message()

		name := message.AttributeNamed("name").Value.String()
		values := message.AttributeNamed("values").Value.List()

		traitsMap[name] = &internal.Value{
			Type:  internal.TypeList,
			Value: values,
		}
	}

	return &internal.Value{
		Type: internal.TypeMap,
		Value: &internal.MapValue{
			Elems: traitsMap,
		},
	}, nil
}

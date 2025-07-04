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

package unmarshaling

import (
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/trace"
)

var _ bot.UnmarshalConfigContext = (ContextFunc)(nil)

// ContextFunc wraps an UnmarshalDestination function to satisfy the
// bot.UnmarshalConfigContext interface.
type ContextFunc func(node *yaml.Node) (destination.Destination, error)

// UnmarshalDestination implements bot.UnmarshalConfigContext.
func (fn ContextFunc) UnmarshalDestination(node *yaml.Node) (destination.Destination, error) {
	return fn(node)
}

// ExtractDestination implements bot.UnmarshalConfigContext.
func (fn ContextFunc) ExtractDestination(node *yaml.Node) (destination.Destination, error) {
	for i, subNode := range node.Content {
		if subNode.Value == "destination" {
			// Next node will be the contents
			dest, err := fn(node.Content[i+1])
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Remove key and contents from root node
			node.Content = slices.Delete(node.Content, i, i+2)
			return dest, nil
		}
	}
	return nil, nil
}

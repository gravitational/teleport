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

package config

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

// extractOutputDestination performs surgery on yaml.Node to unmarshal a
// destination and then remove key/values from the yaml.Node that specify
// the destination. This *hack* allows us to have the bot.Destination as a
// field directly on an Output without needing a struct to wrap it for
// polymorphic unmarshaling.
//
// If there's no destination in the provided yaml node, then this will return
// nil, nil.
func extractOutputDestination(node *yaml.Node) (bot.Destination, error) {
	for i, subNode := range node.Content {
		if subNode.Value == "destination" {
			// Next node will be the contents
			dest, err := unmarshalDestination(node.Content[i+1])
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Remove key and contents from root node
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return dest, nil
		}
	}
	return nil, nil
}

func validateOutputDestination(dest bot.Destination) error {
	if dest == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := dest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating configured destination")
	}
	return nil
}

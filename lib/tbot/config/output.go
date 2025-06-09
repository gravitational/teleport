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
	"io"

	"github.com/goccy/go-yaml/ast"

	"github.com/gravitational/trace"

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
func extractOutputDestination(file *ast.File) (bot.Destination, error) {
	if len(file.Docs) != 1 {
		return nil, trace.BadParameter("multiple docs is bad")
	}
	mapNode, ok := file.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, trace.BadParameter("expected map node")
	}
	for i, kv := range mapNode.Values {
		key := kv.Key.String()
		if key == "destination" {
			destBytes, err := io.ReadAll(kv.Value)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Next node will be the contents
			dest, err := unmarshalDestination(destBytes)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Remove key and contents from root node
			mapNode.Values = append(mapNode.Values[:i], mapNode.Values[i+1:]...)
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

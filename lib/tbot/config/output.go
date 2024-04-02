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

// Output is an interface that represents configurable Outputs for a bot.
// These outputs are the core unit of generating artifacts in tbot and are the
// element users configure to control what is output.
type Output interface {
	// CheckAndSetDefaults validates the configuration and sets any defaults.
	//
	// This must be called before other methods on Output can be called as the
	// implementations may depend on the default values.
	CheckAndSetDefaults() error
	// MarshalYAML enables the yaml package to correctly marshal the Output as
	// YAML.
	MarshalYAML() (interface{}, error)

	GetDestination() bot.Destination
}

// ListSubdirectories lists all subdirectories that will be used by the given
// templates. Primarily used for on-the-fly directory creation.
func listSubdirectories(templates []template) ([]string, error) {
	// Note: currently no standard identity.Artifacts create subdirs. If that
	// ever changes, we'll need to adapt this to ensure we initialize them
	// properly on the fly.
	var subDirs []string

	for _, t := range templates {
		for _, file := range t.describe() {
			if file.IsDir {
				subDirs = append(subDirs, file.Name)
			}
		}
	}

	return subDirs, nil
}

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

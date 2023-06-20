/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

// Output is an interface that represents configurable Outputs for a bot.
// These outputs are the core unit of generating artifacts in tbot and are the
// element users configure to control what is output.
type Output interface {
	// GetDestination returns the bot.Destination that the Output writing to.
	//
	// This can be useful for extracting content that has been written in
	// tests or as part of the `tbot init` command.
	GetDestination() bot.Destination
	// CheckAndSetDefaults validates the configuration and sets any defaults.
	//
	// This must be called before other methods on Output can be called as the
	// implementations may depend on the default values.
	CheckAndSetDefaults() error
	// GetRoles returns the roles configured for that Output so that the
	// tbot.Bot the Output belongs to knows what impersonated identity to pass
	// to Render.
	//
	// This will eventually be removed as we move more logic into the Outputs.
	GetRoles() []string
	// Render executes the Output with the given identity and provider, causing
	// the Output to write to the configured bot.Destination.
	Render(context.Context, provider, *identity.Identity) error
	// Init instructs the Output to initialize its underlying bot.Destination.
	// Typical Init activities include creating any necessary folders or
	// initializing in-memory maps.
	//
	// This must be called before Render.
	Init() error
	// MarshalYAML enables the yaml package to correctly marshal the Output as
	// YAML.
	MarshalYAML() (interface{}, error)
	// Describe returns a list of all files that will be created by an Output,
	// this enables commands like `tbot init` to pre-create and configure these
	// files with the correct permissions
	Describe() []FileDescription
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

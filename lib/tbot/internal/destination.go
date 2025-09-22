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

package internal

import (
	"io"
	"slices"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot/destination"
)

// UnmarshalConfigContext is passed to the UnmarshalConfig method implemented by
// service config structs. It provides a way to unmarshal fields that may be
// dynamically registered (like the Kubernetes Secret Destination, which is only
// available if you import the k8s package) without maintaining a global registry.
type UnmarshalConfigContext interface {
	DestinationUnmarshaler
}

// DefaultDestinationUnmarshaler implements the DestinationUnmarshaler interface
// using only the built-in destinations (Memory and Directory). Other destinations
// with heavy dependencies (e.g. Kubernetes Secrets) must be handled in the outer
// tbot/config package to avoid bloat for consumers of the bot package.
//
// It's separate from the tbot/config package's implementation so that it can
// be used in service tests.
type DefaultDestinationUnmarshaler struct{}

// UnmarshalDestination implements DestinationUnmarshaler.
func (DefaultDestinationUnmarshaler) UnmarshalDestination(node *yaml.Node) (destination.Destination, error) {
	header := struct {
		Type string `yaml:"type"`
	}{}
	if err := node.Decode(&header); err != nil {
		return nil, trace.Wrap(err)
	}

	switch header.Type {
	case destination.MemoryType:
		v := &destination.Memory{}
		if err := node.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	case destination.DirectoryType:
		v := &destination.Directory{}
		if err := node.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	default:
		return nil, trace.BadParameter("unrecognized destination type (%s)", header.Type)
	}
}

type DestinationUnmarshaler interface {
	UnmarshalDestination(node *yaml.Node) (destination.Destination, error)
}

// ExtractOutputDestination performs surgery on yaml.Node to unmarshal a
// destination and then remove key/values from the yaml.Node that specify
// the destination. This *hack* allows us to have the destination.Destination
// as a field directly on an Output without needing a struct to wrap it for
// polymorphic unmarshaling.
//
// If there's no destination in the provided yaml node, then this will return
// nil, nil.
func ExtractOutputDestination(unmarshaler DestinationUnmarshaler, node *yaml.Node) (destination.Destination, error) {
	for i, subNode := range node.Content {
		if subNode.Value == "destination" {
			// Next node will be the contents
			dest, err := unmarshaler.UnmarshalDestination(node.Content[i+1])
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

// UnmarshalYAMLConfig is a convenience method for unmarshaling YAML into a
// config struct of type T. If *T implements the UnmarshalConfig method, it
// will be called with the DefaultDestinationUnmarshaler - otherwise it will
// be unmarshaled with UnmarshalYAML (or the default unmarshaling).
//
// It is mainly intended for use in tests.
func UnmarshalYAMLConfig[T any](reader io.Reader) (*T, error) {
	decoder := yaml.NewDecoder(reader)

	var (
		target    T
		targetPtr any = &target
		err       error
	)
	if cu, ok := targetPtr.(configUnmarshaler); ok {
		err = decoder.Decode(&unmarshalConfigWrapper{cu})
	} else {
		err = decoder.Decode(&target)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &target, nil
}

type configUnmarshaler interface {
	UnmarshalConfig(UnmarshalConfigContext, *yaml.Node) error
}

type unmarshalConfigWrapper struct{ unmarshaler configUnmarshaler }

func (w *unmarshalConfigWrapper) UnmarshalYAML(node *yaml.Node) error {
	return w.unmarshaler.UnmarshalConfig(DefaultDestinationUnmarshaler{}, node)
}

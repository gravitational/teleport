/*
Copyright 2022 Gravitational, Inc.

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
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/tool/tbot/botfs"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

type SymlinksMode string

const (
	// SymlinksInsecure does allow resolving symlink paths and does not issue
	// any symlink-related warnings.
	SymlinksInsecure SymlinksMode = "insecure"

	// SymlinksTrySecure attempts to write files securely and avoid symlink
	// attacks, but falls back with a warning if the necessary OS / kernel
	// support is missing.
	SymlinksTrySecure SymlinksMode = "try-secure"

	// SymlinksSecure attempts to write files securely and fails with an error
	// if the operation fails. This should be the default on systems were we
	// expect it to be supported.
	SymlinksSecure SymlinksMode = "secure"
)

// DestinationDirectory is a Destination that writes to the local filesystem
type DestinationDirectory struct {
	Path     string       `yaml:"path,omitempty"`
	Symlinks SymlinksMode `yaml:"symlinks,omitempty"`
}

func (dd *DestinationDirectory) UnmarshalYAML(node *yaml.Node) error {
	// Accept either a string path or a full struct (allowing for options in
	// the future, e.g. configuring permissions, etc):
	//   directory: /foo
	// or:
	//   directory:
	//     path: /foo
	//     some_future_option: bar

	var path string
	if err := node.Decode(&path); err == nil {
		dd.Path = path
		return nil
	}

	// Shenanigans to prevent UnmarshalYAML from recursing back to this
	// override (we want to use standard unmarshal behavior for the full
	// struct)
	type rawDirectory DestinationDirectory
	return trace.Wrap(node.Decode((*rawDirectory)(dd)))
}

func (dd *DestinationDirectory) CheckAndSetDefaults() error {
	if dd.Path == "" {
		return trace.BadParameter("destination path must not be empty")
	}

	secureSupported, err := botfs.IsCreateSecureSupported()
	if err != nil {
		return trace.Wrap(err)
	}

	switch dd.Symlinks {
	case "":
		if secureSupported {
			// We expect Openat2 to be available, so try to use it by default.
			dd.Symlinks = SymlinksSecure
		} else {
			// TrySecure will print a warning on fallback.
			dd.Symlinks = SymlinksTrySecure
		}
	case SymlinksInsecure, SymlinksTrySecure:
		// valid
	case SymlinksSecure:
		if !secureSupported {
			return trace.BadParameter("symlink mode %q not supported on this system", secureSupported)
		}
	default:
		return trace.BadParameter("invalid symlinks mode: %q", dd.Symlinks)
	}

	return nil
}

func (dd *DestinationDirectory) Write(name string, data []byte) error {
	if err := os.WriteFile(filepath.Join(dd.Path, name), data, botfs.DefaultMode); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (dd *DestinationDirectory) Read(name string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(dd.Path, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

func (dd *DestinationDirectory) String() string {
	return fmt.Sprintf("directory %s", dd.Path)
}

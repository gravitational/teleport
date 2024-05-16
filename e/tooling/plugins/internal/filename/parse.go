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
package filename

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

var (
	filenamePattern *regexp.Regexp = regexp.MustCompile(`^(?P<plugin>.*)-teleport-v(?P<version>.*)-(?P<os>linux|darwin|windows)-(?P<arch>amd64|arm|arm64)-bin.tar.gz$`)
)

// Info holds information about a plugin, deduced from from its Houston-compatible
// filename.
type Info struct {
	// Type represents the plugin type, e.g. "terraform-provider"
	Type string
	// Version holds the parsed plugin version number
	Version semver.Version
	// OS is the operating system the plugin was built for
	OS string
	// Arch is the CPU architecture the plugin was built for
	Arch string
}

// Parse attempts to deduce information about a staged plugin from its (assumed
// Houston-compatible) filename, returning an error if the filename can't be
// parsed.
func Parse(filename string) (Info, error) {
	filename = filepath.Base(filename)

	matches := filenamePattern.FindStringSubmatch(filename)
	if len(matches) == 0 {
		return Info{}, trace.Errorf("filename %q does not match required pattern", filename)
	}

	version, err := semver.NewVersion(matches[2])
	if err != nil {
		return Info{}, trace.Wrap(err, "failed parsing version as semver")
	}

	return Info{
		Type:    matches[1],
		Version: *version,
		OS:      matches[3],
		Arch:    matches[4],
	}, nil
}

// Filename generates a Houston-compatible filename for the Info block, with a
// given file extension (NB: the extension is expected to include the leading
// dot).
func (info *Info) Filename(extension string) string {
	return fmt.Sprintf("%s-teleport-v%s-%s-%s-bin%s", info.Type, info.Version, info.OS, info.Arch, extension)
}

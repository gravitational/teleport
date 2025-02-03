// Teleport
// Copyright (C) 2025  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-ref-generator/reference"
)

const configHelp string = `The path to a YAML configuration file with the following fields:

## Main config

required_field_types: a list of type info mappings (see "Type info") indicating
type names of fields that must be present in a dynamic resource before we
include it in the reference. For example, if this is "Metadata" from package
"types", a struct type must include a field with the a field of "types.Metadata"
before we add it to the reference.

source (string): the path to the root of a Go project directory.

destination (string): the path to a directory where the generator writes
reference pages.

excluded_resource_types: a list of type info mappings (see "Type info")
indicating names of resources to exclude from the reference. 

field_assignment_method: the name of a method of a resource type that assigns
fields to the resource. Used to identify the kind and version of a resource.

## Type info

package: The path of a Go package
name: The name of a type within the package

## Example

required_types:
  - name: Metadata
    package: api/types
  - name: ResourceHeader
    package: api/types
source: "api"
destination: "docs/pages/includes/resource-reference.mdx"
excluded_resource_types:
  - package: "types"
    name: "ResourceHeader"
field_assignment_method: "setStaticFields"
`

func main() {
	conf := flag.String("config", "conf.yaml", configHelp)
	flag.Parse()

	conffile, err := os.Open(*conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open the configuration file %v: %v\n", *conf, err)
		os.Exit(1)
	}
	genconf := reference.GeneratorConfig{}
	if err := yaml.NewDecoder(conffile).Decode(&genconf); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration file: %v\n", err)
		os.Exit(1)
	}

	osFS := afero.NewOsFs()
	err = reference.Generate(osFS, osFS, genconf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not generate the resource reference: %v\n", err)
		os.Exit(1)
	}
}

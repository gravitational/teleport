// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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
	"gen-resource-ref/reference"
	"os"

	"gopkg.in/yaml.v3"
)

const help string = `the path to a YAML configuration file with the following fields:

## Main config

required_types (string): a list of type info mappings (see "Type info")
source (string): the path to the root of a Go project directory
destination (string): the file path of the resource reference

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
`

func main() {
	conf := flag.String("config", "./conf.yaml", help)
	flag.Parse()

	conffile, err := os.Open(*conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open the configuration file %v: %v\n", *conf, err)
		os.Exit(1)
	}

	genconf := reference.GeneratorConfig{}
	if err = yaml.NewDecoder(conffile).Decode(&genconf); err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse the configuration file: %v\n", err)
		os.Exit(1)
	}

	if err := genconf.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration file: %v\n", err)
		os.Exit(1)
	}

	outfile, err := os.Create(genconf.DestinationPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create the output file: %v\n", err)
		os.Exit(1)
	}

	err = reference.Generate(outfile, genconf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not generate the resource reference: %v\n", err)
		os.Exit(1)
	}
}

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

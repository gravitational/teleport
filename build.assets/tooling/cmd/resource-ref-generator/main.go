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
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-ref-generator/reference"
)

var tmplPath = filepath.Join("reference", tmplBase)

const (
	tmplBase              = "reference.tmpl"
	configHelp            = `The path to a YAML configuration file (see the README)`
	teleportPackagePrefix = "github.com/gravitational/teleport"
)

func main() {
	conf := flag.String("config", "conf.yaml", configHelp)
	flag.Parse()

	conffile, err := os.Open(*conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open the configuration file %v: %v\n", *conf, err)
		os.Exit(1)
	}
	defer conffile.Close()

	var genconf reference.GeneratorConfig
	if err := yaml.NewDecoder(conffile).Decode(&genconf); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration file: %v\n", err)
		os.Exit(1)
	}
	tmpl, err := template.New(tmplBase).ParseFiles(tmplPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open resource reference template at %v: %v\n", tmplPath, err)
		os.Exit(1)
	}

	err = reference.Generate(teleportPackagePrefix, genconf, tmpl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not generate the resource reference: %v\n", err)
		os.Exit(1)
	}
}

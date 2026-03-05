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

	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/config-ref-generator/generator"
)

const (
	tmplBase   = "generator.tmpl"
	configHelp = "Path to a YAML configuration file"
)

func main() {
	confPath := flag.String("config", "config.yaml", configHelp)
	flag.Parse()

	confFile, err := os.Open(*confPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open config file %v: %v\n", *confPath, err)
		os.Exit(1)
	}
	defer confFile.Close()

	var conf generator.GeneratorConfig
	if err := yaml.NewDecoder(confFile).Decode(&conf); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config file: %v\n", err)
		os.Exit(1)
	}

	tmplPath := filepath.Join(filepath.Dir(*confPath), "generator", tmplBase)
	tmpl, err := generator.NewTemplate(tmplPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot load template at %v: %v\n", tmplPath, err)
		os.Exit(1)
	}

	if err := generator.Generate(conf, tmpl); err != nil {
		fmt.Fprintf(os.Stderr, "generation failed: %v\n", err)
		os.Exit(1)
	}
}

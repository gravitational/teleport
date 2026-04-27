// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

func run() error {
	app := kingpin.New(
		"benchfind",
		"Find Go packages that define benchmarks without compiling test binaries.",
	)
	app.HelpFlag.Short('h')

	tags := app.Flag("tags", "Comma-separated build tags.").Short('t').String()
	cwd := app.Flag("directory", "Working directory to run package discovery from. (Default: current directory)").Short('d').ExistingDir()
	excludes := app.Flag("exclude", "List of package prefixes to skip.").Short('e').Strings()
	patterns := app.Arg("patterns", `Package patterns. (Default: "./...")`).Default("./...").Strings()
	kingpin.MustParse(app.Parse(os.Args[1:]))

	dir := *cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return trace.Wrap(err, "failed to get current working directory")
		}
	}

	cfg := Config{
		Patterns:  *patterns,
		BuildTags: *tags,
		Dir:       dir,
		Excludes:  *excludes,
	}

	pkgs, err := Find(cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, p := range pkgs {
		fmt.Println(p)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		if trace.IsBadParameter(err) {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintln(os.Stderr, trace.DebugReport(err))
		}
		os.Exit(1)
	}
}

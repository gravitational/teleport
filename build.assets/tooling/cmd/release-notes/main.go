/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin/v2"
)

var (
	version   = kingpin.Arg("version", "Version to be released").Required().String()
	changelog = kingpin.Arg("changelog", "Path to CHANGELOG.md").Required().String()
)

func main() {
	kingpin.Parse()

	clFile, err := os.Open(*changelog)
	if err != nil {
		log.Fatal(err)
	}
	defer clFile.Close()

	gen := &releaseNotesGenerator{
		releaseVersion: *version,
	}

	notes, err := gen.generateReleaseNotes(clFile)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(notes)
}

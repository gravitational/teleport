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

// Command print-import-path prints the import path that
// should appear in Go import paths to stdout.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gravitational/teleport/build.assets/gomod"

	"github.com/gravitational/trace"
)

// prints the import path of the api module
func main() {
	if len(os.Args) < 1 {
		log.Fatal("first argument should be a path to a go.mod file")
	}
	goModFilePath := os.Args[1]

	modPath, err := gomod.GetImportPath(goModFilePath)
	if err != nil {
		log.Fatal(trace.Wrap(err))
	}
	fmt.Println(modPath)
}

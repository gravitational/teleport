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
package main

import (
	"os"
	"strings"

	"github.com/gravitational/kingpin"
)

type args struct {
	providerTag           string
	registryURL           string
	artifactDirectoryPath string
	registryDirectoryPath string
	signingKeyText        string
	protocolVersions      []string
	providerNamespace     string
	providerName          string
	verbosity             int
}

func parseCommandLine() *args {
	app := kingpin.New("promote-terraform", "Adds files to a terraform registry")
	result := &args{}

	app.Flag("tag", "The version tag identifying  version of provider to promote").
		Required().
		StringVar(&result.providerTag)

	app.Flag("artifact-directory-path", "The path to a directory that contains the Terraform provider artifacts").
		Required().
		ExistingDirVar(&result.artifactDirectoryPath)

	app.Flag("registry-directory-path", "The path to a local copy of the registry").
		Required().
		ExistingDirVar(&result.registryDirectoryPath)

	app.Flag("signing-key", "GPG signing key in ASCII armor format").
		Short('k').
		Envar("SIGNING_KEY").
		StringVar(&result.signingKeyText)

	app.Flag("protocol", "Terraform protocol supported by files").
		Short('p').
		Default("4.0", "5.1").
		StringsVar(&result.protocolVersions)

	app.Flag("registry-url", "Address where registry objects will be served.").
		Default("https://terraform.releases.teleport.dev/").
		StringVar(&result.registryURL)

	app.Flag("namespace", "Terraform provider namespace").
		Default("gravitational").
		StringVar(&result.providerNamespace)

	app.Flag("name", "Terraform provider name").
		Default("teleport").
		StringVar(&result.providerName)

	app.Flag("verbose", "Output more trace output").
		Short('v').
		CounterVar(&result.verbosity)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Marshal the arguments into a canonical format here, so we don't have to
	// second guess the format later on when we're in the thick of doing the
	// actual work...

	if !strings.HasSuffix(result.registryURL, "/") {
		result.registryURL = result.registryURL + "/"
	}

	return result
}

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
	"os/exec"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type CreateRepo struct {
	cacheDir   string
	binaryName string
}

// Instantiates createrepo, ensuring all system requirements for performing createrepo operations
// have been met
func NewCreateRepo(cacheDir string) (*CreateRepo, error) {
	cr := &CreateRepo{
		cacheDir: cacheDir,
		// `createrepo_c` is the "new" (as in 9 years old) replacement for `createrepo`
		// This can be replace with "createrepo" in the unlikely chance that there is
		//  a problem
		binaryName: "createrepo_c",
	}

	err := cr.ensureBinaryExists()
	if err != nil {
		return nil, trace.Wrap(err, "failed to ensure CreateRepo binary exists")
	}

	// Ensure the cache dir exists
	err = os.MkdirAll(cr.cacheDir, 0660)
	if err != nil {
		return nil, trace.Wrap(err, "failed to ensure %q exists", cr.cacheDir)
	}

	return cr, nil
}

func (cr *CreateRepo) ensureBinaryExists() error {
	_, err := exec.LookPath(cr.binaryName)
	if err != nil {
		return trace.Wrap(err, "failed to verify that %q binary exists", cr.binaryName)
	}

	return nil
}

func (cr *CreateRepo) CreateOrUpdateRepo(repoPath string) error {
	// <cr.binaryName> --cachedir <cr.cacheDir> --update <repoPath>
	logrus.Debugf("Updating repo metadata for repo at %q", repoPath)

	args := []string{
		"--cachedir",
		cr.cacheDir,
		"--update",
		repoPath,
	}

	_, err := BuildAndRunCommand(cr.binaryName, args...)
	if err != nil {
		return trace.Wrap(err, "createrepo create/update command failed on path %q with cache directory %q", repoPath, cr.cacheDir)
	}

	return nil
}

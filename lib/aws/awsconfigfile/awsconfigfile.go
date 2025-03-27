// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package awsconfigfile

import (
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	ini "gopkg.in/ini.v1"
)

// AWSConfigFilePath returns the path to the AWS configuration file.
func AWSConfigFilePath() (string, error) {
	if location := os.Getenv("AWS_CONFIG_FILE"); location != "" {
		return location, nil
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return filepath.Join(homedir, ".aws", "config"), nil
}

// SetDefaultProfileCredentialProcess sets the credential_process for the default profile.
func SetDefaultProfileCredentialProcess(configFilePath string, credentialProcess string) error {
	sectionName := "default"
	return trace.Wrap(addCredentialProcessToSection(configFilePath, sectionName, credentialProcess))
}

// UpsertProfileCredentialProcess sets the credential_process for the profile with name profileName.
// File is created if it does not exist
func UpsertProfileCredentialProcess(configFilePath string, profileName string, credentialProcess string) error {
	sectionName := "profile " + profileName
	return trace.Wrap(addCredentialProcessToSection(configFilePath, sectionName, credentialProcess))
}

func addCredentialProcessToSection(configFilePath string, sectionName string, credentialProcess string) error {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		AllowNestedValues: true, // Allow AWS-like nested values. Docs: http://docs.aws.amazon.com/cli/latest/topic/config-vars.html#nested-values
		Loose:             true, // Allow non-existing files. ini.SaveTo will create the file if it does not exist.
	}, configFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	if !iniFile.HasSection(sectionName) {
		iniFile.NewSection(sectionName)
	}

	defaultSection := iniFile.Section(sectionName)
	defaultSection.NewKey("credential_process", credentialProcess)

	if len(defaultSection.KeyStrings()) > 1 {
		return trace.BadParameter("default section contains other keys: %v", defaultSection.KeyStrings())
	}

	// Create the directory if it does not exist, otherwise ini.SaveTo will fail.
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o755); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

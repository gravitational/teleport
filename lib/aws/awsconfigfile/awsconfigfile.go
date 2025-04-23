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
	// See https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
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
// File is created if it does not exist.
func SetDefaultProfileCredentialProcess(configFilePath string, credentialProcess string) error {
	sectionName := "default"
	return trace.Wrap(addCredentialProcessToSection(configFilePath, sectionName, credentialProcess))
}

// UpsertProfileCredentialProcess sets the credential_process for the profile with name profileName.
// File is created if it does not exist.
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

	section := iniFile.Section(sectionName)
	section.Comment = "This section is managed by Teleport. Do not edit."
	_, err = section.NewKey("credential_process", credentialProcess)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(section.KeyStrings()) > 1 {
		return trace.BadParameter("default section contains other keys: %v", section.KeyStrings())
	}

	// Create the directory if it does not exist, otherwise ini.SaveTo will fail.
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o755); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

// RemoveProfilesUsingCredentialProcess removes all profiles that use the given credential_process.
func RemoveProfilesUsingCredentialProcess(configFilePath string, credentialProcess string) error {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		AllowNestedValues: true,  // Allow AWS-like nested values. Docs: http://docs.aws.amazon.com/cli/latest/topic/config-vars.html#nested-values
		Loose:             false, // If file does not exist, then there's nothing to be removed.
	}, configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Ignore non-existing file.
			return nil
		}

		return trace.Wrap(err)
	}

	sectionChanged := false
	for _, section := range iniFile.Sections() {
		if section.HasKey("credential_process") && section.Key("credential_process").String() == credentialProcess {
			sectionChanged = true
			iniFile.DeleteSection(section.Name())
		}
	}

	// No need to save the file if no sections were changed.
	if !sectionChanged {
		return nil
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

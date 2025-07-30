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
	"cmp"
	"os"
	"path/filepath"
	"strings"

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
func SetDefaultProfileCredentialProcess(configFilePath, sectionComment, credentialProcess string) error {
	const sectionName = "default"
	return trace.Wrap(addCredentialProcessToSection(configFilePath, sectionName, sectionComment, credentialProcess))
}

// UpsertProfileCredentialProcess sets the credential_process for the profile with name profileName.
// File is created if it does not exist.
func UpsertProfileCredentialProcess(configFilePath, profileName, sectionComment, credentialProcess string) error {
	sectionName := "profile " + profileName
	return trace.Wrap(addCredentialProcessToSection(configFilePath, sectionName, sectionComment, credentialProcess))
}

func addCredentialProcessToSection(configFilePath, sectionName, sectionComment, credentialProcessCommand string) error {
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
	if cmp.Or(section.Comment, sectionComment) != sectionComment {
		return trace.BadParameter("%s: section %q is not managed by teleport, remove the section and try again", configFilePath, sectionName)
	}

	section.Comment = sectionComment
	_, err = section.NewKey("credential_process", credentialProcessCommand)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(section.KeyStrings()) > 1 {
		return trace.BadParameter("%s: section %q contains other keys, remove the section and try again", configFilePath, sectionName)
	}

	// Create the directory if it does not exist, otherwise ini.SaveTo will fail.
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o755); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

// RemoveCredentialProcessByComment removes the credential_process key on sections that have a specific section comment.
func RemoveCredentialProcessByComment(configFilePath, sectionComment string) error {
	if !strings.HasPrefix(sectionComment, "; ") {
		sectionComment = "; " + sectionComment
	}

	compareExactlyFn := func(comment string) bool {
		return sectionComment == comment
	}

	return removeCredentialProcess(configFilePath, compareExactlyFn)
}

// RemoveCredentialProcessByCommentPrefix removes the credential_process key on sections that have a specific section comment prefix.
func RemoveCredentialProcessByCommentPrefix(configFilePath, sectionComment string) error {
	if !strings.HasPrefix(sectionComment, "; ") {
		sectionComment = "; " + sectionComment
	}

	comparePrefixFn := func(comment string) bool {
		return strings.HasPrefix(comment, sectionComment)
	}
	return removeCredentialProcess(configFilePath, comparePrefixFn)
}

// RemoveCredentialProcessByComment removes the credential_process key on sections that have a specific section comment.
func removeCredentialProcess(configFilePath string, matchCommentFn func(string) bool) error {
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
		if !matchCommentFn(section.Comment) {
			continue
		}

		if !section.HasKey("credential_process") {
			continue
		}

		section.DeleteKey("credential_process")
		if len(section.Keys()) > 0 {
			return trace.BadParameter("%s: section %q contains other keys, remove the section and try again", configFilePath, section.Name())
		}
		iniFile.DeleteSection(section.Name())

		sectionChanged = true
	}

	// No need to save the file if no sections were changed.
	if !sectionChanged {
		return nil
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

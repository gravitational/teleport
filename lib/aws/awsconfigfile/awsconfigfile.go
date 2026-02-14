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
	"strings"

	"github.com/gravitational/trace"
	ini "gopkg.in/ini.v1"
)

const (
	ownershipComment    = "Do not edit. Section managed by Teleport."
	ssoOwnershipComment = "Do not edit. Section managed by Teleport (AWS Identity Center integration)."
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

// SSOSession represents an AWS SSO session configuration.
type SSOSession struct {
	// Name is the session name.
	Name string
	// StartURL is the SSO start URL.
	StartURL string
	// Region is the SSO region.
	Region string
}

// WriteSSOConfig writes multiple SSO profiles and sessions to the AWS configuration file in a single pass,
// and prunes any stale Teleport-managed SSO sections.
func WriteSSOConfig(configFilePath string, profiles []SSOProfile, sessions []SSOSession) error {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		AllowNestedValues: true,
		Loose:             true,
	}, configFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	keepProfiles := make(map[string]struct{})
	keepSessions := make(map[string]struct{})

	// Add/Update sessions.
	for _, s := range sessions {
		sectionName := "sso-session " + s.Name
		keepSessions[sectionName] = struct{}{}

		section, err := getOrCreateManagedSection(iniFile, sectionName, ssoOwnershipComment)
		if err != nil {
			return trace.Wrap(err)
		}
		section.Comment = ssoOwnershipComment
		if _, err := section.NewKey("sso_start_url", s.StartURL); err != nil {
			return trace.Wrap(err)
		}
		if _, err := section.NewKey("sso_region", s.Region); err != nil {
			return trace.Wrap(err)
		}
	}

	// Add/Update profiles.
	for _, p := range profiles {
		sectionName := "profile " + p.Name
		keepProfiles[sectionName] = struct{}{}

		section, err := getOrCreateManagedSection(iniFile, sectionName, ssoOwnershipComment)
		if err != nil {
			return trace.Wrap(err)
		}

		if section.HasKey("credential_process") {
			return trace.BadParameter("%s: section %q contains 'credential_process' and cannot be converted to an SSO profile, remove the section and try again", configFilePath, section.Name())
		}

		section.Comment = ssoOwnershipComment
		if _, err := section.NewKey("sso_session", p.Session); err != nil {
			return trace.Wrap(err)
		}
		if _, err := section.NewKey("sso_account_id", p.AccountID); err != nil {
			return trace.Wrap(err)
		}
		if _, err := section.NewKey("sso_role_name", p.RoleName); err != nil {
			return trace.Wrap(err)
		}
	}

	// Prune stale sections.
	for _, section := range iniFile.Sections() {
		if !strings.Contains(section.Comment, ssoOwnershipComment) {
			continue
		}

		name := section.Name()
		shouldDelete := false
		switch {
		case strings.HasPrefix(name, "profile "):
			if _, ok := keepProfiles[name]; !ok {
				shouldDelete = true
			}
		case strings.HasPrefix(name, "sso-session "):
			if _, ok := keepSessions[name]; !ok {
				shouldDelete = true
			}
		}

		if shouldDelete {
			iniFile.DeleteSection(name)
		}
	}

	// Create the directory if it does not exist.
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o700); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

func getOrCreateManagedSection(iniFile *ini.File, sectionName, expectedComment string) (*ini.Section, error) {
	if iniFile.HasSection(sectionName) {
		section := iniFile.Section(sectionName)
		if !strings.Contains(section.Comment, expectedComment) {
			return nil, trace.BadParameter("section %q is not managed by Teleport, remove the section and try again", sectionName)
		}
		return section, nil
	}

	section, err := iniFile.NewSection(sectionName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return section, nil
}

// UpsertSSOSession sets the sso_start_url and sso_region for the sso-session with name sessionName.
func UpsertSSOSession(configFilePath, sessionName, ssoStartURL, ssoRegion string) error {
	return trace.Wrap(upsertManagedSSOSection(configFilePath, "sso-session "+sessionName, func(section *ini.Section) error {
		if _, err := section.NewKey("sso_start_url", ssoStartURL); err != nil {
			return trace.Wrap(err)
		}
		if _, err := section.NewKey("sso_region", ssoRegion); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}))
}

// SSOProfile represents an AWS Identity Center profile configuration.
type SSOProfile struct {
	// Name is the profile name.
	Name string
	// Session is the name of the SSO session.
	Session string
	// AccountID is the AWS account ID.
	AccountID string
	// RoleName is the name of the IAM role.
	RoleName string
	// Account is the AWS account name or alias.
	Account string
}

// UpsertSSOProfile sets the sso_session, sso_account_id and sso_role_name for the profile with name profileName.
func UpsertSSOProfile(configFilePath string, p SSOProfile) error {
	return trace.Wrap(upsertManagedSSOSection(configFilePath, "profile "+p.Name, func(section *ini.Section) error {
		if section.HasKey("credential_process") {
			return trace.BadParameter("%s: section %q contains 'credential_process' and cannot be converted to an SSO profile, remove the section and try again", configFilePath, section.Name())
		}
		if _, err := section.NewKey("sso_session", p.Session); err != nil {
			return trace.Wrap(err)
		}
		if _, err := section.NewKey("sso_account_id", p.AccountID); err != nil {
			return trace.Wrap(err)
		}
		if _, err := section.NewKey("sso_role_name", p.RoleName); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}))
}

func upsertManagedSSOSection(configFilePath, sectionName string, updateFunc func(*ini.Section) error) error {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		AllowNestedValues: true,
		Loose:             true,
	}, configFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	var section *ini.Section
	// If section exists, verify ownership and bail if not owned by Teleport.
	if iniFile.HasSection(sectionName) {
		section = iniFile.Section(sectionName)
		if !strings.Contains(section.Comment, ssoOwnershipComment) {
			return trace.BadParameter("%s: section %q is not managed by Teleport, remove the section and try again", configFilePath, sectionName)
		}
	} else {
		section, err = iniFile.NewSection(sectionName)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	section.Comment = ssoOwnershipComment
	if err := updateFunc(section); err != nil {
		return trace.Wrap(err)
	}

	// Create the directory if it does not exist.
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o700); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

// SetDefaultProfileCredentialProcess sets the credential_process for the default profile.
// File is created if it does not exist.
func SetDefaultProfileCredentialProcess(configFilePath, credentialProcess string) error {
	const sectionName = "default"
	return trace.Wrap(addCredentialProcessToSection(configFilePath, sectionName, credentialProcess))
}

// UpsertProfileCredentialProcess sets the credential_process for the profile with name profileName.
// File is created if it does not exist.
func UpsertProfileCredentialProcess(configFilePath, profileName, credentialProcess string) error {
	sectionName := "profile " + profileName
	return trace.Wrap(addCredentialProcessToSection(configFilePath, sectionName, credentialProcess))
}

func addCredentialProcessToSection(configFilePath, sectionName, credentialProcessCommand string) error {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		AllowNestedValues: true, // Allow AWS-like nested values. Docs: http://docs.aws.amazon.com/cli/latest/topic/config-vars.html#nested-values
		Loose:             true, // Allow non-existing files. ini.SaveTo will create the file if it does not exist.
	}, configFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	var section *ini.Section

	switch {
	case iniFile.HasSection(sectionName):
		section = iniFile.Section(sectionName)

		if !strings.Contains(section.Comment, ownershipComment) {
			return trace.BadParameter("%s: section %q is not managed by Teleport, remove the section and try again", configFilePath, sectionName)
		}

	default:
		section, err = iniFile.NewSection(sectionName)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	section.Comment = ownershipComment
	_, err = section.NewKey("credential_process", credentialProcessCommand)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(section.KeyStrings()) > 1 {
		return trace.BadParameter("%s: section %q contains other keys, remove the section and try again", configFilePath, sectionName)
	}

	// Create the directory if it does not exist, otherwise ini.SaveTo will fail.
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o700); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

// RemoveTeleportManagedProfile removes the credential_process key on sections that have a specific section comment.
func RemoveTeleportManagedProfile(configFilePath, profile string) error {
	sectionName := "profile " + profile

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

	configFileChanged := false
	if iniFile.HasSection(sectionName) {
		section := iniFile.Section(sectionName)

		if !strings.Contains(section.Comment, ownershipComment) {
			return trace.BadParameter("%s: section %q is not managed by Teleport, remove the section manually and try again", configFilePath, sectionName)
		}

		if strings.Contains(section.Comment, ownershipComment) {
			iniFile.DeleteSection(section.Name())
			configFileChanged = true
		}
	}
	// No need to save the file if no sections were changed.
	if !configFileChanged {
		return nil
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

// RemoveAllTeleportManagedProfiles removes all the profiles managed by Teleport.
func RemoveAllTeleportManagedProfiles(configFilePath string) error {
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
		if strings.Contains(section.Comment, ownershipComment) {
			iniFile.DeleteSection(section.Name())
			sectionChanged = true
		}
	}

	// No need to save the file if no sections were changed.
	if !sectionChanged {
		return nil
	}

	return trace.Wrap(iniFile.SaveTo(configFilePath))
}

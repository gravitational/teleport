/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/aws/awsconfigfile"
	"github.com/gravitational/teleport/lib/tlsca"
)

// writeFilesForExternalApps create or update local configuration files to allow external applications to use the credentials generated by tsh apps login.
// These files are outside of the tsh profile directory.
func writeFilesForExternalApps(appInfo *appInfo) error {
	if appInfo.RouteToApp.AWSCredentialProcessCredentials != "" {
		if err := addAWSProfileToConfig(appInfo.RouteToApp.Name); err != nil {
			return trace.Wrap(err, "failed to add AWS profile to config")
		}
	}

	return nil
}

// removeExternalFilesForApp remove or update local configuration files that allowed external applications to use the credentials generated by tsh apps login.
// These files are outside of the tsh profile directory.
func removeExternalFilesForApp(routeToApp tlsca.RouteToApp) error {
	if routeToApp.AWSCredentialProcessCredentials != "" {
		if err := removeAWSProfileFromConfig(routeToApp.Name); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// removeExternalFilesForAllApps remove or update local configuration files that allowed external applications to use the credentials generated by tsh apps login.
// These files are outside of the tsh profile directory.
func removeExternalFilesForAllApps() error {
	if err := removeAllAWSProfilesFromConfig(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// This value is used to identify the section in the AWS config file that is managed by Teleport.
// If it changes, the removal of the section during logout will not work correctly.
func awsConfigCommentForProfile(appName string) string {
	return fmt.Sprintf("Do not edit. Section managed by Teleport. Generated for accessing %s", appName)
}

// addAWSProfileToConfig adds a new profile to the AWS config file (ie. ~/.aws/config).
// This profile will invoke the `tsh apps config --format aws-credential-process <app name>` command
// which returns the credentials for accessing AWS.
func addAWSProfileToConfig(appName string) error {
	awsConfigFileLocation, err := awsconfigfile.AWSConfigFilePath()
	if err != nil {
		return trace.Wrap(err)
	}

	credentialProcessCommand := fmt.Sprintf("tsh apps config --format aws-credential-process %s", appName)
	sectionComment := awsConfigCommentForProfile(appName)
	profileName := appName

	if err := awsconfigfile.UpsertProfileCredentialProcess(awsConfigFileLocation, profileName, sectionComment, credentialProcessCommand); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func removeAWSProfileFromConfig(appName string) error {
	awsConfigFileLocation, err := awsconfigfile.AWSConfigFilePath()
	if err != nil {
		return trace.Wrap(err)
	}

	sectionComment := awsConfigCommentForProfile(appName)

	return trace.Wrap(awsconfigfile.RemoveCredentialProcessByComment(awsConfigFileLocation, sectionComment))
}

func removeAllAWSProfilesFromConfig() error {
	awsConfigFileLocation, err := awsconfigfile.AWSConfigFilePath()
	if err != nil {
		return trace.Wrap(err)
	}

	sectionComment := awsConfigCommentForProfile("")

	return trace.Wrap(awsconfigfile.RemoveCredentialProcessByComment(awsConfigFileLocation, sectionComment))
}

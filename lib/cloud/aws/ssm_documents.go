/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package aws

import (
	"fmt"

	"github.com/google/uuid"
)

// EC2DiscoverySSMDocumentOptions are options for generating the EC2 SSM discovery document.
type EC2DiscoverySSMDocumentOptions struct {
	// InsecureSkipInstallPathRandomization skips randomizing the Teleport installation script file path.
	InsecureSkipInstallPathRandomization bool
}

// WithInsecureSkipInstallPathRandomization returns an option func that
// sets the InsecureSkipInstallPathRandomization option.
func WithInsecureSkipInstallPathRandomization(setting bool) func(*EC2DiscoverySSMDocumentOptions) {
	return func(options *EC2DiscoverySSMDocumentOptions) {
		options.InsecureSkipInstallPathRandomization = setting
	}
}

// EC2DiscoverySSMDocument receives the proxy address and returns an SSM Document.
// This document downloads and runs a Teleport installer.
// Requires the proxy endpoint URL, example: https://tenant.teleport.sh
func EC2DiscoverySSMDocument(proxy string, opts ...func(*EC2DiscoverySSMDocumentOptions)) string {
	var options EC2DiscoverySSMDocumentOptions
	for _, optFn := range opts {
		optFn(&options)
	}

	installTeleportPath := "/tmp/installTeleport.sh"
	if !options.InsecureSkipInstallPathRandomization {
		// Randomize the install path so the filename can not be guessed to avoid possible script injection
		installTeleportPath = fmt.Sprintf("/tmp/installTeleport-%s.sh", uuid.NewString())
	}

	return fmt.Sprintf(`
schemaVersion: '2.2'
description: aws:runShellScript
parameters:
  token:
    type: String
    description: "(Required) The Teleport invite token to use when joining the cluster."
  scriptName:
    type: String
    description: "(Required) The Teleport installer script to use when joining the cluster."
mainSteps:
- action: aws:downloadContent
  name: downloadContent
  inputs:
    sourceType: "HTTP"
    destinationPath: %q
    sourceInfo:
      url: "%s/webapi/scripts/installer/{{ scriptName }}"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - /bin/sh %s "{{ token }}"
`, installTeleportPath, proxy, installTeleportPath)
}

const EC2DiscoveryPolicyName = "TeleportEC2Discovery"

// EC2DiscoverySSMDocumentSteps is the list of Steps defined in the default SSM Document for Teleport Discovery.
// Used to query step results after executing a command using SSM.
var EC2DiscoverySSMDocumentSteps = []string{
	"downloadContent",
	"runShellScript",
}

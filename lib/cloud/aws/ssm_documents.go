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

// EC2DiscoverySSMDocument receives the proxy address and returns an SSM Document.
// This document downloads and runs a Teleport installer.
// Requires the proxy endpoint URL, example: https://tenant.teleport.sh
func EC2DiscoverySSMDocument(proxy string) string {
	randString := uuid.NewString() // Secure random so the filename can not be guessed to avoid possible script injection

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
    destinationPath: "/tmp/installTeleport-%s.sh"
    sourceInfo:
      url: "%s/webapi/scripts/installer/{{ scriptName }}"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - /bin/sh /tmp/installTeleport-%s.sh "{{ token }}"
`, randString, proxy, randString)
}

const EC2DiscoveryPolicyName = "TeleportEC2Discovery"

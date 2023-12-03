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

import "fmt"

func EC2DiscoverySSMDocument(proxy string) string {
	return fmt.Sprintf(ec2DiscoverySSMDocument, proxy)
}

const EC2DiscoveryPolicyName = "TeleportEC2Discovery"

const ec2DiscoverySSMDocument = `
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
    destinationPath: "/tmp/installTeleport.sh"
    sourceInfo:
      url: "%s/webapi/scripts/installer/{{ scriptName }}"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - /bin/sh /tmp/installTeleport.sh "{{ token }}"
`

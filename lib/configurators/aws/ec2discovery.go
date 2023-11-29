// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"time"
)

func EC2DiscoverySSMDocument(proxy string) string {
	randomBytes := make([]byte, 4)
	if _, err := crand.Read(randomBytes); err != nil {
		// on error from crypto rand fallback to less secure math random
		mathRand := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		binary.LittleEndian.PutUint32(randomBytes, mathRand.Uint32())
	}
	randString := hex.EncodeToString(randomBytes)

	return fmt.Sprintf(ec2DiscoverySSMDocument, randString, proxy, randString)
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
    destinationPath: "/tmp/installTeleport-%s.sh"
    sourceInfo:
      url: "%s/webapi/scripts/installer/{{ scriptName }}"
- action: aws:runShellScript
  name: runShellScript
  inputs:
    timeoutSeconds: '300'
    runCommand:
      - /bin/sh /tmp/installTeleport-%s.sh "{{ token }}"
`

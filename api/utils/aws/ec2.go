/*
Copyright 2023 Gravitational, Inc.

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

package aws

import (
	"regexp"
)

// EC2 Node IDs are {AWS account ID}-{EC2 resource ID} eg:
//
//	123456789012-i-1234567890abcdef0
//
// AWS account ID is always a 12 digit number, see
//
//	https://docs.aws.amazon.com/general/latest/gr/acct-identifiers.html
//
// EC2 resource ID is i-{8 or 17 hex digits}, see
//
//	https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/resource-ids.html
var ec2NodeIDRE = regexp.MustCompile("^[0-9]{12}-i-[0-9a-f]{8,}$")

// IsEC2NodeID returns true if the given ID looks like an EC2 node ID. Uses a
// simple regex to check. Node IDs are almost always UUIDs when set
// automatically, but can be manually overridden by admins. If someone manually
// sets a host ID that looks like one of our generated EC2 node IDs, they may be
// able to trick this function, so don't use it for any critical purpose.
func IsEC2NodeID(id string) bool {
	return ec2NodeIDRE.MatchString(id)
}

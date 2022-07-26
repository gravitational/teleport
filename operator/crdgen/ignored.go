/*
Copyright 2022 Gravitational, Inc.

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

package main

import "github.com/Clever/go-utils/stringset"

/*
Fields that we are ignoring when creating a CRD
Each entry represents the ignore fields using the resource name as the version

One of the reasons to ignore fields those fields is because they are readonly in Teleport
CRD do not support readonly logic
This should be removed when the following feature is implemented
https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#transition-rules
*/
var ignoredFields = map[string]stringset.StringSet{
	"UserSpecV2": stringset.New("LocalAuth", "Expires", "CreatedBy", "Status"),
}

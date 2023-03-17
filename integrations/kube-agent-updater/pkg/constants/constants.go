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

package constants

import "time"

const (
	// MaintenancePath is the version discovery endpoint representing if the
	// target version represents a critical update as defined in
	// https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md#version-discovery-endpoint
	MaintenancePath = "critical"
	// VersionPath is the version discovery endpoint returning the current
	// target version as defined in
	// https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md#version-discovery-endpoint
	VersionPath   = "version"
	HTTPTimeout   = 10 * time.Second
	CacheDuration = time.Minute
)

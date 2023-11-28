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

package controller

import (
	"fmt"
)

// MaintenanceNotTriggeredError indicates that no trigger returned true and the controller did not reconcile.
type MaintenanceNotTriggeredError struct {
	Message string `json:"message"`
}

// Error returns log friendly description of an error
func (e *MaintenanceNotTriggeredError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "maintenance not triggered"
}

// NoNewVersionError indicates that no new version was found and the controller did not reconcile.
type NoNewVersionError struct {
	Message        string `json:"message"`
	CurrentVersion string `json:"currentVersion"`
	NextVersion    string `json:"nextVersion"`
}

// Error returns log friendly description of an error
func (e *NoNewVersionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("no new version (current: %q, next: %q)", e.CurrentVersion, e.NextVersion)
}

// IncompatibleVersionError indicates that the target version is incompatible with the auth server version
type IncompatibleVersionError struct {
	Message     string `json:"message"`
	AuthVersion string `json:"authVersion"`
	NextVersion string `json:"nextVersion"`
}

// Error returns a log friendly description of an error
func (e *IncompatibleVersionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("next version is incompatible with auth version (auth: %q, next: %q)", e.AuthVersion, e.NextVersion)
}

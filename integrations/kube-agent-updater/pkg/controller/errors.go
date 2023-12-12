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

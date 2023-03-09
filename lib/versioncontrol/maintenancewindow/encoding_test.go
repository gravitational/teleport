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

package maintenancewindow

import (
	"testing"
	"time"
)

type kubeScheduleRepr struct {
	Windows []windowRepr `json:"windows"`
}

type windowRepr struct {
	Start time.Time `json:"start"`
	Stop  time.Time `json:"stop"`
}

// TestKubeSchedule verifies that the json representation of the agent schedule remains
// consistent so that we don't break compat with old upgraders.
func TestKubeSchedule(t *testing.T) {
	panic("TODO")
}

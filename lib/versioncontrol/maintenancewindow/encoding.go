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
	"fmt"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// EncodeKubeControllerSchedule converts an agent upgrade schedule to the file format
// expected by the kuberenets upgrade controller.
func EncodeKubeControllerSchedule(schedule types.AgentUpgradeSchedule) (string, error) {
	b, err := utils.FastMarshal(&schedule)
	if err != nil {
		return "", trace.Errorf("failed to encode kube controller schedule: %v", err)
	}

	return string(b), nil
}

// unitScheduleHeader is the first line in the systemd unit upgrader schedule. The teleport-upgrade
// script invoked by the unit ignores all lines starting with '# '.
const unitScheduleHeader = "# periodically exported by teleport, modifications are not preserved"

// EncodeSystemdUnitSchedule converts an agent upgrade schedule to the file format
// expected by the teleport-upgrade script.
func EncodeSystemdUnitSchedule(schedule types.AgentUpgradeSchedule) (string, error) {
	if len(schedule.Windows) == 0 {
		return "", trace.BadParameter("cannot encode empty schedule")
	}
	lines := make([]string, 0, len(schedule.Windows)+1)
	lines = append(lines, unitScheduleHeader)
	for _, window := range schedule.Windows {
		// upgrade windows are encoded as a pair of space-separated unix timestamps.
		lines = append(lines, fmt.Sprintf("%d %d", window.Start.Unix(), window.Stop.Unix()))
	}

	return strings.Join(lines, "\n"), nil
}

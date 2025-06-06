// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"fmt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/darwinbundle"
	"github.com/gravitational/teleport/lib/utils/log/oslog"
)

func getPlatformInitLoggerOpts(cf *CLIConf) []utils.LoggerOption {
	if !cf.OSLog {
		return nil
	}

	subsystem, err := darwinbundle.Identifier()
	if err != nil {
		subsystem = "tsh"

		// The actual logger isn't initialized yet, so let's log the error to os_log.
		logger := oslog.NewLogger(subsystem, teleport.ComponentTSH)
		message := fmt.Sprintf("Could not get bundle identifier to set it as os_log subsystem, using 'tsh' as a fallback error=%v", err)
		logger.Log(oslog.OsLogTypeDebug, message)
	}

	return []utils.LoggerOption{utils.WithOSLog(subsystem)}
}

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package discovery

import (
	"errors"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/gravitational/teleport/api/types/usertasks"
)

// classifyAzureVMEnrollmentError classifies Azure API errors into user-facing
// messages for VM auto-discovery. This is best-effort based on error strings
// which may change without notice. The matching logic may require future
// adjustments to track upstream changes, as well as expansion to handle new
// error patterns. Unrecognized errors will return a generic message; the user
// should check server logs for the underlying error details.
func classifyAzureVMEnrollmentError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		switch {
		case respErr.StatusCode == 403 && respErr.ErrorCode == "AuthorizationFailed":
			if strings.Contains(errMsg, "runCommands") {
				return usertasks.AutoDiscoverAzureVMIssueMissingRunCommandsPermission
			}
		case respErr.StatusCode == 409 && respErr.ErrorCode == "OperationNotAllowed":
			if strings.Contains(errMsg, "VM is not running") {
				return usertasks.AutoDiscoverAzureVMIssueVMNotRunning
			}
			if strings.Contains(errMsg, "extension operations are disallowed") {
				return usertasks.AutoDiscoverAzureVMIssueVMAgentNotAvailable
			}
		}
	}

	// generic error
	return usertasks.AutoDiscoverAzureVMIssueEnrollmentError
}

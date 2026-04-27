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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/usertasks"
)

func TestClassifyAzureVMEnrollmentError(t *testing.T) {
	azureError := func(statusCode int, errorCode, errorMessage string) error {
		resp := &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(errorMessage)),
			Request:    &http.Request{Method: http.MethodPut, URL: &url.URL{}},
		}
		return runtime.NewResponseErrorWithErrorCode(resp, errorCode)
	}

	tests := []struct {
		name              string
		err               error
		expectedIssueType string
	}{
		{
			name:              "nil error",
			err:               nil,
			expectedIssueType: "",
		},
		{
			name:              "unrecognized error",
			err:               errors.New("something went wrong"),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueEnrollmentError,
		},
		{
			name:              "403 AuthorizationFailed with runCommands/write",
			err:               azureError(403, "AuthorizationFailed", "does not have authorization to perform action 'Microsoft.Compute/virtualMachines/runCommands/write'"),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueMissingRunCommandsPermission,
		},
		{
			name:              "403 AuthorizationFailed with runCommands/read",
			err:               azureError(403, "AuthorizationFailed", "does not have authorization to perform action 'Microsoft.Compute/virtualMachines/runCommands/read'"),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueMissingRunCommandsPermission,
		},
		{
			name:              "403 AuthorizationFailed without runCommands",
			err:               azureError(403, "AuthorizationFailed", "does not have authorization to perform some other action"),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueEnrollmentError,
		},
		{
			name:              "409 OperationNotAllowed VM not running",
			err:               azureError(409, "OperationNotAllowed", "Cannot modify extensions in the VM when the VM is not running"),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueVMNotRunning,
		},
		{
			name:              "409 OperationNotAllowed extension operations disallowed",
			err:               azureError(409, "OperationNotAllowed", "This operation cannot be performed when extension operations are disallowed. To allow, please ensure VM Agent is installed on the VM and the osProfile.allowExtensionOperations property is true."),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueVMAgentNotAvailable,
		},
		{
			name:              "409 OperationNotAllowed other message",
			err:               azureError(409, "OperationNotAllowed", "some other operation not allowed error"),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueEnrollmentError,
		},
		{
			name:              "500 InternalServerError",
			err:               azureError(500, "InternalServerError", "An internal error occurred"),
			expectedIssueType: usertasks.AutoDiscoverAzureVMIssueEnrollmentError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyAzureVMEnrollmentError(tt.err)
			require.Equal(t, tt.expectedIssueType, result, "issue type mismatch")
		})
	}
}

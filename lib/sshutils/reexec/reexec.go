/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package reexec contains a common implementation for teleport reexec commands.
package reexec

import (
	"errors"
	"fmt"
	"io"
	"os/user"
	"strings"

	"github.com/gravitational/trace"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

const maxRead = 4096

// ErrorContext contains context used to enrich child process launch errors.
type ErrorContext struct {
	// DecisionContext contains RBAC decision details used to clarify
	// access related launch failures.
	DecisionContext *decisionpb.SSHAccessPermitContext
	// Login is the target OS login used by the child process.
	Login string
}

// ReadChildError reads the child process's stderr pipe and returns it as a string,
// potentially with additional error context gathered from the given ErrorContext.
// If the stderr pipe is empty, an empty string and nil error is returned.
func ReadChildError(stderr io.Reader, context *ErrorContext) (string, error) {
	// Read the error msg from stderr.
	errMsg := new(strings.Builder)
	if _, err := io.Copy(errMsg, io.LimitReader(stderr, maxRead)); err != nil {
		return "", trace.Wrap(err, "failed to read error message from child process")
	}

	if errMsg.Len() == 0 {
		return "", nil
	}

	// If we don't have a decision context, we don't have any context to
	// add to the error message. Return stderr as is.
	if context == nil || context.DecisionContext == nil {
		return errMsg.String(), nil
	}

	// If some roles allow host user creation while others deny it, this can be
	// ambiguous to the end user and warrants clarification if it results in an
	// unknown user error.
	ambiguousHostUserDenial := len(context.DecisionContext.HostUserCreationDeniedBy) > 0 && len(context.DecisionContext.HostUserCreationAllowedBy) > 0
	ambiguousHostUserError := func() string {
		var deniedBy []string
		for _, d := range context.DecisionContext.HostUserCreationDeniedBy {
			deniedBy = append(deniedBy, fmt.Sprintf("%v: %q", d.Kind, d.Name))
		}
		return fmt.Sprintf("%s: host user creation denied by the following resources: [%s]\n", strings.TrimRight(errMsg.String(), ".\n"), strings.Join(deniedBy, ", "))
	}

	unknownUserError := user.UnknownUserError(context.Login)
	switch {
	case strings.Contains(errMsg.String(), "failed to open PAM context"): // PAM errors are often cause by an unknown user.
		if _, err := user.Lookup(context.Login); errors.Is(err, unknownUserError) {
			if ambiguousHostUserDenial {
				return ambiguousHostUserError(), nil
			}
		}
	case strings.Contains(errMsg.String(), unknownUserError.Error()):
		if ambiguousHostUserDenial {
			return ambiguousHostUserError(), nil
		}
	}

	return errMsg.String(), nil
}

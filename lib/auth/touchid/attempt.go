/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package touchid

import (
	"errors"

	"github.com/gravitational/trace"

	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// ErrAttemptFailed is returned by AttemptLogin and AttemptDeleteNonInteractive
// for attempts that failed before user interaction.
type ErrAttemptFailed struct {
	// Err is the underlying failure for the attempt.
	Err error
}

func (e *ErrAttemptFailed) Error() string {
	return e.Err.Error()
}

func (e *ErrAttemptFailed) Unwrap() error {
	return e.Err
}

func (e *ErrAttemptFailed) Is(target error) bool {
	_, ok := target.(*ErrAttemptFailed)
	return ok
}

func (e *ErrAttemptFailed) As(target any) bool {
	tt, ok := target.(*ErrAttemptFailed)
	if ok {
		tt.Err = e.Err
		return true
	}
	return false
}

// AttemptLogin attempts a touch ID login.
// It returns ErrAttemptFailed if the attempt failed before user interaction.
// See Login.
func AttemptLogin(origin, user string, assertion *wantypes.CredentialAssertion, picker CredentialPicker) (*wantypes.CredentialAssertionResponse, string, error) {
	resp, actualUser, err := Login(origin, user, assertion, picker)
	switch {
	case errors.Is(err, ErrNotAvailable), errors.Is(err, ErrCredentialNotFound):
		return nil, "", &ErrAttemptFailed{Err: err}
	case err != nil:
		return nil, "", trace.Wrap(err)
	}
	return resp, actualUser, nil
}

// AttemptDeleteNonInteractive attempts to delete a Secure Enclave credential.
// Does not require user interaction.
func AttemptDeleteNonInteractive(credentialID string) error {
	if !IsAvailable() {
		return &ErrAttemptFailed{Err: ErrNotAvailable}
	}
	if credentialID == "" {
		return trace.BadParameter("credentialID required")
	}
	switch err := native.DeleteNonInteractive(credentialID); {
	case errors.Is(err, ErrCredentialNotFound):
		return &ErrAttemptFailed{Err: err}
	case err != nil:
		return trace.Wrap(err)
	}
	return nil
}

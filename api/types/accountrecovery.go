/*
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"time"

	"github.com/gravitational/trace"
)

// NewRecoveryCodes creates a new RecoveryCodes with the given codes and created
// time.
func NewRecoveryCodes(codes []RecoveryCode, created time.Time, username string) (*RecoveryCodesV1, error) {
	rc := &RecoveryCodesV1{
		Metadata: Metadata{
			Name: username,
		},
		Spec: RecoveryCodesSpecV1{
			Codes:   codes,
			Created: created,
		},
	}

	if err := rc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return rc, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (t *RecoveryCodesV1) CheckAndSetDefaults() error {
	t.setStaticFields()

	if err := t.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if t.Spec.Codes == nil {
		return trace.BadParameter("missing Codes field")
	}

	if t.Spec.Created.IsZero() {
		return trace.BadParameter("missing Created field")
	}

	return nil
}

func (t *RecoveryCodesV1) setStaticFields() {
	t.Kind = KindRecoveryCodes
	t.Version = V1
}

// GetCodes returns recovery codes.
func (t *RecoveryCodesV1) GetCodes() []RecoveryCode {
	return t.Spec.Codes
}

// RecoveryAttempt represents an unsuccessful attempt at recovering a user's account.
type RecoveryAttempt struct {
	// Time is time of the attempt.
	Time time.Time `json:"time"`
	// Expires defines the time when this attempt should expire.
	Expires time.Time `json:"expires"`
}

func (a *RecoveryAttempt) Check() error {
	switch {
	case a.Time.IsZero():
		return trace.BadParameter("missing parameter time")
	case a.Expires.IsZero():
		return trace.BadParameter("missing parameter expires")
	}
	return nil
}

// IsMaxFailedRecoveryAttempt determines if user reached their max failed attempts.
// Attempts list is expected to come sorted from oldest to latest time.
func IsMaxFailedRecoveryAttempt(maxAttempts int, attempts []*RecoveryAttempt, now time.Time) bool {
	var failed int
	for i := len(attempts) - 1; i >= 0; i-- {
		if attempts[i].Expires.After(now) {
			failed++
		}
		if failed >= maxAttempts {
			return true
		}
	}
	return false
}

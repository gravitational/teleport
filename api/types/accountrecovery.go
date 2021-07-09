/**
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

// NewRecoveryCodes creates a new RecoveryCodes with the given codes and created time.
func NewRecoveryCodes(codes []RecoveryCode, created time.Time) (*RecoveryCodes, error) {
	rc := &RecoveryCodes{
		Codes:   codes,
		Created: created,
	}

	if err := rc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return rc, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (t *RecoveryCodes) CheckAndSetDefaults() error {
	t.setStaticFields()

	if t.Codes == nil {
		return trace.BadParameter("missing Codes field")
	}

	if t.Created.IsZero() {
		return trace.BadParameter("missing Created field")
	}

	return nil
}

func (t *RecoveryCodes) setStaticFields() {
	t.Kind = KindRecoveryCodes
	t.Version = V1
}

func (t *RecoveryCodes) GetKind() string               { return t.Kind }
func (t *RecoveryCodes) GetVersion() string            { return t.Version }
func (t *RecoveryCodes) GetCodes() []RecoveryCode      { return t.Codes }
func (t *RecoveryCodes) SetCreation(created time.Time) { t.Created = created }

// RecoveryAttempt represents an unsuccessful attempt at recovering a user's account.
type RecoveryAttempt struct {
	// Time is time of the attempt.
	Time time.Time `json:"time"`
	// Expires defines the time when this attempt should expire.
	Expires time.Time `json:"expires"`
}

func (a *RecoveryAttempt) Check() error {
	if a.Time.IsZero() {
		return trace.BadParameter("missing parameter time")
	}

	if a.Expires.IsZero() {
		return trace.BadParameter("missing parameter expires")
	}

	return nil
}

// SortedRecoveryAttempts sorts recovery attempts by time.
type SortedRecoveryAttempts []RecoveryAttempt

// Len returns length of a role list.
func (s SortedRecoveryAttempts) Len() int {
	return len(s)
}

// Less stacks latest attempts to the end of the list.
func (s SortedRecoveryAttempts) Less(i, j int) bool {
	return s[i].Time.Before(s[j].Time)
}

// Swap swaps two attempts.
func (s SortedRecoveryAttempts) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// LastFailedRecoveryAttempt determines if user reached their max failed attempts.
func LastFailedRecoveryAttempt(x int, attempts []RecoveryAttempt, now time.Time) bool {
	var failed int
	for i := len(attempts) - 1; i >= 0; i-- {
		if attempts[i].Expires.After(now) {
			failed++
		}
		if failed >= x {
			return true
		}
	}
	return false
}

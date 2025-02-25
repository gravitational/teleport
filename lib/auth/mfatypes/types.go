/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package mfatypes

import mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"

// ChallengeExtensions is a json struct for [mfav1.ChallengeExtensions].
type ChallengeExtensions struct {
	Scope                       mfav1.ChallengeScope      `json:"scope"`
	AllowReuse                  mfav1.ChallengeAllowReuse `json:"allow_reuse,omitempty"`
	UserVerificationRequirement string                    `json:"user_verification_requirement,omitempty"`
}

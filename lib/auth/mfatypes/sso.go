/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import (
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
)

// BeginSSOMFAChallengeParams contains parameters for lib/auth/Server.BeginSSOMFAChallenge. This struct is in this
// package in order to avoid a circular dependency between lib/auth and lib/auth/mfa/mfav1.
// TODO(cthach): Move params struct back to lib/auth package after SSO MFA device support is added to lib/auth/authtest
// (https://github.com/gravitational/teleport/issues/62271) and the lib/auth/mfa/mfav1.AuthServer interface is updated.
type BeginSSOMFAChallengeParams struct {
	User                 string
	SSO                  *types.SSOMFADevice
	SSOClientRedirectURL string
	ProxyAddress         string
	Ext                  *mfav1.ChallengeExtensions
	SIP                  *mfav1.SessionIdentifyingPayload
	SourceCluster        string
	TargetCluster        string
}

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

package proxy

import (
	"net/http"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// kubernetesSessionTerminatedByUser is the message that is sent to the
	// client when the session is terminated by the moderator.
	kubernetesSessionTerminatedByModerator = "Session terminated by moderator."
	sessionTerminatedByModeratorReason     = metav1.StatusReason("SessionTerminatedByModerator")
)

var sessionTerminatedByModeratorErr = &kubeerrors.StatusError{
	ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    http.StatusUnauthorized,
		Reason:  sessionTerminatedByModeratorReason,
		Message: kubernetesSessionTerminatedByModerator,
		Details: &metav1.StatusDetails{
			Causes: []metav1.StatusCause{
				{
					Type:    metav1.CauseTypeForbidden,
					Message: kubernetesSessionTerminatedByModerator,
				},
			},
		},
	},
}

// isSessionTerminatedError returns true if the error is a session terminated error.
// This is required because StreamWithContext wraps the error into a new error string
// and we lose the type information to forward the error to the client.
func isSessionTerminatedError(err error) bool {
	if err == nil {
		return false
	}
	// This check is required because the error is wrapped into a new error string
	// by StreamWithContext and we lose the type information.
	return err.Error() == kubernetesSessionTerminatedByModerator
}

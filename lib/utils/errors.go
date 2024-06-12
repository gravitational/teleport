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

package utils

import (
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// IsUseOfClosedNetworkError returns true if the specified error
// indicates the use of a closed network connection.
func IsUseOfClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), constants.UseOfClosedNetworkConnection)
}

// IsFailedToSendCloseNotifyError returns true if the provided error is the
// "tls: failed to send closeNotify".
func IsFailedToSendCloseNotifyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), constants.FailedToSendCloseNotify)
}

// IsOKNetworkError returns true if the provided error received from a network
// operation is one of those that usually indicate normal connection close. If
// the error is a trace.Aggregate, all the errors must be OK network errors.
func IsOKNetworkError(err error) bool {
	// trace.Aggregate contains at least one error and all the errors are
	// non-nil
	if a, ok := trace.Unwrap(err).(trace.Aggregate); ok {
		for _, err := range a.Errors() {
			if !IsOKNetworkError(err) {
				return false
			}
		}
		return true
	}
	return errors.Is(err, io.EOF) || IsUseOfClosedNetworkError(err) || IsFailedToSendCloseNotifyError(err)
}

// IsConnectionRefused returns true if the given err is "connection refused" error.
func IsConnectionRefused(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ECONNREFUSED
	}
	return false
}

// IsExpiredCredentialError checks if an error corresponds to expired credentials.
func IsExpiredCredentialError(err error) bool {
	return IsHandshakeFailedError(err) || IsCertExpiredError(err) || trace.IsBadParameter(err) || trace.IsTrustError(err)
}

// IsUntrustedCertErr checks if an error is an untrusted cert error.
func IsUntrustedCertErr(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "x509: certificate is valid for") ||
		strings.Contains(errMsg, "certificate is not trusted")
}

const (
	// SelfSignedCertsMsg is a helper message to point users towards helpful documentation.
	SelfSignedCertsMsg = "Your proxy certificate is not trusted or expired. " +
		"Please update the certificate or follow this guide for self-signed certs: https://goteleport.com/docs/management/admin/self-signed-certs/"
)

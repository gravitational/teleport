//go:build piv && !pivtest

// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package piv

import (
	"strconv"
	"strings"
)

// TODO(Joerger): Both piv and pc/sc errors would be better handled through their
// represented error codes rather than their associated message. PR and/or fork
// upstream piv-go library to expose these error codes for more stable error checks.

// pivErrCode is a hexadecimal error code returned according to the PIV specification.
// https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73pt2-5.pdf#page=25 - Section 3.2.1.1
type pivErrCode int

const (
	// PIN, Touch, or some other security authorization is required and was not provided.
	pivErrCodeSecurityStatusNotSatisfied pivErrCode = 0x6982
)

// isPIVErrorCode returns whether this error pertains to a PIV error code.
func isPIVErrorCode(err error, errCode pivErrCode) bool {
	// The piv-go library wraps the error code as a string with a user readable message.
	// e.g. "smart card error 6982: security status not satisfied"
	return strings.Contains(err.Error(), strconv.FormatInt(int64(errCode), 16))
}

// pcsc error messages are returned from the piv-go library for specific pcsc error codes.
// https://pcsclite.apdu.fr/api/group__ErrorCodes.html
const (
	// The smart card connection was closed/reset mid operation.
	//
	// On Windows, we have observed this error occurring on signatures that take longer than a few seconds,
	// usually due to an unanswered PIN prompt.
	pcscErrMsgResetCard = "the smart card has been reset, so any shared state information is invalid"
)

func isPCSCError(err error, pcscErrMsg string) bool {
	return strings.Contains(err.Error(), pcscErrMsg)
}

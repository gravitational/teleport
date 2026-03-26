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

package mfa

import (
	"fmt"

	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

const (
	// mfaModeAuto automatically chooses the best MFA device(s), without any
	// restrictions.
	MFAModeAuto = "auto"
	// MFAModeCrossPlatform utilizes only cross-platform devices, such as
	// pluggable hardware keys.
	// Implies Webauthn.
	MFAModeCrossPlatform = "cross-platform"
	// MFAModePlatform utilizes only platform devices, such as Touch ID.
	// Implies Webauthn.
	MFAModePlatform = "platform"
	// MFAModeSSO utilizes only SSO devices.
	MFAModeSSO = "sso"
	// MFAModeBrowser utilizes browser-based WebAuthn MFA.
	MFAModeBrowser = "browser"
)

type MFAModeOpts struct {
	AuthenticatorAttachment wancli.AuthenticatorAttachment
	PreferSSO               bool
	PreferBrowser           bool
}

func ParseMFAMode(mode string) (*MFAModeOpts, error) {
	opts := &MFAModeOpts{}
	switch mode {
	case "", MFAModeAuto:
	case MFAModeCrossPlatform:
		opts.AuthenticatorAttachment = wancli.AttachmentCrossPlatform
	case MFAModePlatform:
		opts.AuthenticatorAttachment = wancli.AttachmentPlatform
	case MFAModeSSO:
		opts.PreferSSO = true
	case MFAModeBrowser:
		opts.PreferBrowser = true
	default:
		return nil, fmt.Errorf("invalid MFA mode: %q", mode)
	}
	return opts, nil
}

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

package lite

import (
	"fmt"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

func setDirectoryPermissions(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return trace.Wrap(err, "getting the SID of the current user")
	}
	// https://learn.microsoft.com/en-us/windows/win32/secauthz/security-descriptor-definition-language
	// D:P  a protected discretionary ACL, inheritance from the parent is blocked
	// OICI object inherit and container inherit - set permissions for children
	// FA   FILE_ALL_ACCESS
	// SY   LocalSystem
	// BA   built-in Administrators
	sddl := fmt.Sprintf("D:P(A;OICI;FA;;;%s)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", user.User.Sid.String())
	return trace.Wrap(setPermissionsFromSDDL(path, sddl))
}

func setDBPermissions(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return trace.Wrap(err, "getting the SID of the current user")
	}
	// https://learn.microsoft.com/en-us/windows/win32/secauthz/security-descriptor-definition-language
	// D:P  a protected discretionary ACL, inheritance from the parent is blocked
	// FA   FILE_ALL_ACCESS
	// SY   LocalSystem
	// BA   built-in Administrators
	sddl := fmt.Sprintf("D:P(A;;FA;;;%s)(A;;FA;;;SY)(A;;FA;;;BA)", user.User.Sid.String())
	return trace.Wrap(setPermissionsFromSDDL(path, sddl))
}

func setPermissionsFromSDDL(path, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return trace.Wrap(err, "parsing security descriptor %q", sddl)
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return trace.Wrap(err, "getting ACL descriptor")
	}

	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, // owner, group unchanged
		dacl,
		nil, // SACL unchanged
	)

	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "setting permissions on %q", path)
	}

	return nil
}

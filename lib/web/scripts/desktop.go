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

package scripts

import (
	_ "embed"
	"text/template"
)

// desktopAccessInstallADDSScript is the script that will run on the
// windows machine and install Active Directory Domain Services
//
//go:embed desktop/install-ad-ds.ps1
var DesktopAccessScriptInstallADDS string

// desktopAccessInstallADCSScript is the script that will run on the
// windows machine and install Active Directory Certificate Services
//
//go:embed desktop/install-ad-cs.ps1
var DesktopAccessScriptInstallADCS string

// desktopAccessConfigureScript is the script that will run on the windows
// machine and configure Active Directory
//
//go:embed desktop/configure-ad.ps1
var desktopAccessScriptConfigure string
var DesktopAccessScriptConfigure = template.Must(template.New("desktop-access-configure-ad").Parse(desktopAccessScriptConfigure))

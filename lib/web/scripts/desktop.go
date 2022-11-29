/*
Copyright 2015-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

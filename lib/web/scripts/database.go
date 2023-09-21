// Copyright 2023 Gravitational, Inc
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

package scripts

import (
	_ "embed"
	"text/template"
)

//go:embed database/sqlserver/configure-ad.ps1
var databaseAccessSQLServerConfigureScript string

// DatabaseAccessSQLServerConfigureScript is the script that will run on Windows
// machine and configure Active Directory.
var DatabaseAccessSQLServerConfigureScript = template.Must(template.New("database-access-sqlserver-configure-ad").Parse(databaseAccessSQLServerConfigureScript))

// DatabaseAccessSQLServerConfigureParams is the template parameters passed to
// the configure script.
type DatabaseAccessSQLServerConfigureParams struct {
	// CACertPEM PEM-encoded database CA.
	CACertPEM string
	// CACertSHA1 database CA SHA1 checksum.
	CACertSHA1 string
	// CACertBase64 base64-encoded database CA.
	CACertBase64 string
	// CRLPEM PEM-encoded database revocation list.
	CRLPEM string
	// ProxyPublicAddr Teleport proxy public address.
	ProxyPublicAddr string
	// ProvisionToken join token with database permission.
	ProvisionToken string
	// DBAddress database address URI.
	DBAddress string
}

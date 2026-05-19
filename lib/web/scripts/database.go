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

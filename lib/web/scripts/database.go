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

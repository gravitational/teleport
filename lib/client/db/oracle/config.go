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

package oracle

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

type jdbcSettings struct {
	KeyStoreFile       string
	TrustStoreFile     string
	KeyStorePassword   string
	TrustStorePassword string
}

const jdbcPropertiesTemplateContent = `
javax.net.ssl.keyStore={{.KeyStoreFile}}
javax.net.ssl.trustStore={{.TrustStoreFile}}
javax.net.ssl.keyStorePassword={{.KeyStorePassword}}
javax.net.ssl.trustStorePassword={{.TrustStorePassword}}
javax.net.ssl.keyStoreType=jks
javax.net.ssl.trustStoreType=jks
oracle.net.authentication_services=TCPS
`

type tnsNamesORASettings struct {
	ServiceName string
	Host        string
	Port        string
}

const sqlnetORATemplateContent = `
SSL_CLIENT_AUTHENTICATION = TRUE
SQLNET.AUTHENTICATION_SERVICES = (TCPS)

WALLET_LOCATION =
  (SOURCE =
    (METHOD = FILE)
    (METHOD_DATA =
      (DIRECTORY = {{.WalletDir}})
    )
  )
`

type sqlnetORASettings struct {
	WalletDir string
}

const tnsnamesORATemplateContent = `
{{.ServiceName}} =
  (DESCRIPTION =
    (ADDRESS_LIST =
      (ADDRESS = (PROTOCOL = TCPS)(HOST = {{.Host}})(PORT = {{.Port}}))
    )
    (CONNECT_DATA =
      (SERVER = DEDICATED)
      (SERVICE_NAME = {{.ServiceName}})
    )
    (SECURITY =
      (SSL_SERVER_CERT_DN = "CN=localhost")
    )
  )
`

var (
	jdbcPropertiesTemplate = template.Must(template.New("").Parse(jdbcPropertiesTemplateContent))
	sqlnetORATemplate      = template.Must(template.New("").Parse(sqlnetORATemplateContent))
	tnsnamesORATemplate    = template.Must(template.New("").Parse(tnsnamesORATemplateContent))
)

func (c jdbcSettings) template() *template.Template        { return jdbcPropertiesTemplate }
func (c sqlnetORASettings) template() *template.Template   { return sqlnetORATemplate }
func (c tnsNamesORASettings) template() *template.Template { return tnsnamesORATemplate }

func (c jdbcSettings) configFilename() string        { return "ojdbc.properties" }
func (c sqlnetORASettings) configFilename() string   { return "sqlnet.ora" }
func (c tnsNamesORASettings) configFilename() string { return "tnsnames.ora" }

type templateSettings interface {
	template() *template.Template
	configFilename() string
}

func writeSettings(settings templateSettings, dir string) error {
	var buff bytes.Buffer
	if err := settings.template().Execute(&buff, settings); err != nil {
		return trace.Wrap(err)
	}
	filePath := filepath.Join(dir, settings.configFilename())
	if err := os.WriteFile(filePath, buff.Bytes(), teleport.FileMaskOwnerOnly); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

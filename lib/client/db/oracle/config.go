package oracle

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

type jdbcSettings struct {
	KeyStoreFile       string
	TrustStoreFile     string
	KeyStorePassword   string
	TrustStorePassword string
}

var jdbcPropertiesTemplateContent = `
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

var sqlnetORATemplateContent = `
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

var tnsnamesORATemplateContent = `
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

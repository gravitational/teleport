/*
Copyright 2020-2021 Gravitational, Inc.

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

package postgres

import (
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/lib/client/db/profile"

	"github.com/gravitational/trace"
	"gopkg.in/ini.v1"
)

// ServiceFile represents Postgres connection service file.
//
// https://www.postgresql.org/docs/13/libpq-pgservice.html
type ServiceFile struct {
	// iniFile is the underlying ini file.
	iniFile *ini.File
	// path is the service file path.
	path string
}

// Load loads Postgres connection service file from the default location.
func Load() (*ServiceFile, error) {
	// Default location is .pg_service.conf file in the user's home directory.
	// TODO(r0mant): Check PGSERVICEFILE and PGSYSCONFDIR env vars as well.
	user, err := user.Current()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return LoadFromPath(filepath.Join(user.HomeDir, pgServiceFile))
}

// LoadFromPath loads Posrtgres connection service file from the specified path.
func LoadFromPath(path string) (*ServiceFile, error) {
	// Loose load will ignore file not found error.
	iniFile, err := ini.LooseLoad(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServiceFile{
		iniFile: iniFile,
		path:    path,
	}, nil
}

// Upsert adds the provided connection profile to the service file and saves it.
//
// The profile goes into a separate section with the name equal to the
// name of the database that user is logged into and looks like this:
//
//   [postgres]
//   host=proxy.example.com
//   port=3080
//   sslmode=verify-full
//   sslrootcert=/home/user/.tsh/keys/proxy.example.com/certs.pem
//   sslcert=/home/user/.tsh/keys/proxy.example.com/alice-db/root/aurora-x509.pem
//   sslkey=/home/user/.tsh/keys/proxy.example.com/user
//
// With the profile like this, a user can refer to it using "service" psql
// parameter:
//
//   $ psql "service=postgres <other parameters>"
func (s *ServiceFile) Upsert(profile profile.ConnectProfile) error {
	section := s.iniFile.Section(profile.Name)
	if section != nil {
		s.iniFile.DeleteSection(profile.Name)
	}
	section, err := s.iniFile.NewSection(profile.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	section.NewKey("host", profile.Host)
	section.NewKey("port", strconv.Itoa(profile.Port))
	if profile.User != "" {
		section.NewKey("user", profile.User)
	}
	if profile.Database != "" {
		section.NewKey("dbname", profile.Database)
	}
	if profile.Insecure {
		section.NewKey("sslmode", SSLModeVerifyCA)
	} else {
		section.NewKey("sslmode", SSLModeVerifyFull)
	}
	section.NewKey("sslrootcert", profile.CACertPath)
	section.NewKey("sslcert", profile.CertPath)
	section.NewKey("sslkey", profile.KeyPath)
	ini.PrettyFormat = false // Pretty format breaks psql.
	return s.iniFile.SaveTo(s.path)
}

// Env returns the specified connection profile information as a set of
// environment variables recognized by Postgres clients.
func (s *ServiceFile) Env(serviceName string) (map[string]string, error) {
	section, err := s.iniFile.GetSection(serviceName)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, trace.NotFound("service connection profile %q not found", serviceName)
		}
		return nil, trace.Wrap(err)
	}
	host, err := section.GetKey("host")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	port, err := section.GetKey("port")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sslMode, err := section.GetKey("sslmode")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sslRootCert, err := section.GetKey("sslrootcert")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sslCert, err := section.GetKey("sslcert")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sslKey, err := section.GetKey("sslkey")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env := map[string]string{
		"PGHOST":        host.Value(),
		"PGPORT":        port.Value(),
		"PGSSLMODE":     sslMode.Value(),
		"PGSSLROOTCERT": sslRootCert.Value(),
		"PGSSLCERT":     sslCert.Value(),
		"PGSSLKEY":      sslKey.Value(),
	}
	if section.HasKey("user") {
		user, err := section.GetKey("user")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		env["PGUSER"] = user.Value()
	}
	if section.HasKey("dbname") {
		database, err := section.GetKey("dbname")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		env["PGDATABASE"] = database.Value()
	}
	return env, nil
}

// Delete deletes the specified connection profile and saves the service file.
func (s *ServiceFile) Delete(name string) error {
	s.iniFile.DeleteSection(name)
	return s.iniFile.SaveTo(s.path)
}

const (
	// SSLModeVerifyFull is the Postgres SSL "verify-full" mode.
	SSLModeVerifyFull = "verify-full"
	// SSLModeVerifyCA is the Postgres SSL "verify-ca" mode.
	SSLModeVerifyCA = "verify-ca"
	// pgServiceFile is the default name of the Postgres service file.
	pgServiceFile = ".pg_service.conf"
)

// Message is printed after Postgres service file has been updated.
var Message = template.Must(template.New("").Parse(`
Connection information for PostgreSQL database "{{.Name}}" has been saved.

You can now connect to the database using the following command:

  $ psql "service={{.Name}}{{if not .User}} user=<user>{{end}}{{if not .Database}} dbname=<dbname>{{end}}"

Or configure environment variables and use regular CLI flags:

  $ eval $(tsh db env)
  $ psql{{if not .User}} -U <user>{{end}}{{if not .Database}} <dbname>{{end}}

`))

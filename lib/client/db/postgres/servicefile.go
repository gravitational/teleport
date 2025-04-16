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

package postgres

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/ini.v1"

	"github.com/gravitational/teleport/lib/client/db/profile"
)

func init() {
	ini.PrettyFormat = false // Pretty format breaks psql.
}

// ServiceFile represents Postgres connection service file.
//
// https://www.postgresql.org/docs/13/libpq-pgservice.html
type ServiceFile struct {
	// iniFile is the underlying ini file.
	iniFile *ini.File
	// path is the service file path.
	path string
}

// DefaultConfigPath returns the default config path, which is .pg_service.conf
// file in the user's home directory.
func defaultConfigPath() (string, error) {
	// Default location is .pg_service.conf file in the user's home directory.
	// TODO(r0mant): Check PGSERVICEFILE and PGSYSCONFDIR env vars as well.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		usr, err := user.Current()
		if err != nil {
			return "", trace.ConvertSystemError(err)
		}
		home = usr.HomeDir
	}

	return filepath.Join(home, pgServiceFile), nil
}

// Load loads Postgres connection service file from the default location.
func Load() (*ServiceFile, error) {
	cnfPath, err := defaultConfigPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return LoadFromPath(cnfPath)
}

// LoadFromPath loads Postgres connection service file from the specified path.
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
//	[postgres]
//	host=proxy.example.com
//	port=3080
//	sslmode=verify-full
//	sslrootcert=/home/user/.tsh/keys/proxy.example.com/certs.pem
//	sslcert=/home/user/.tsh/keys/proxy.example.com/alice-db/root/aurora-x509.pem
//	sslkey=/home/user/.tsh/keys/proxy.example.com/user
//
// With the profile like this, a user can refer to it using "service" psql
// parameter:
//
//	$ psql "service=postgres <other parameters>"
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
	section.NewKey("gssencmode", "disable") // we dont support GSS encryption.
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
	if section.HasKey("gssencmode") {
		gssEncMode, err := section.GetKey("gssencmode")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		env["PGGSSENCMODE"] = gssEncMode.Value()
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
	//
	// See Postgres SSL docs for more info:
	// https://www.postgresql.org/docs/current/libpq-ssl.html
	SSLModeVerifyFull = "verify-full"
	// SSLModeVerifyCA is the Postgres SSL "verify-ca" mode.
	//
	// See Postgres SSL docs for more info:
	// https://www.postgresql.org/docs/current/libpq-ssl.html
	SSLModeVerifyCA = "verify-ca"
	// pgServiceFile is the default name of the Postgres service file.
	pgServiceFile = ".pg_service.conf"
)

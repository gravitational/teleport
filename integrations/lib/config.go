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

package lib

import (
	"io"
	"os"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

// TeleportConfig stores config options for where
// the Teleport's Auth server is listening, and what certificates to
// use to authenticate in it.
type TeleportConfig struct {
	AuthServer string `toml:"auth_server"`
	Addr       string `toml:"addr"`
	Identity   string `toml:"identity"`
	ClientKey  string `toml:"client_key"`
	ClientCrt  string `toml:"client_crt"`
	RootCAs    string `toml:"root_cas"`
}

func (cfg TeleportConfig) GetAddrs() []string {
	if cfg.Addr != "" {
		return []string{cfg.Addr}
	} else if cfg.AuthServer != "" {
		return []string{cfg.AuthServer}
	}
	return nil
}

func (cfg *TeleportConfig) CheckAndSetDefaults() error {
	if cfg.Addr == "" && cfg.AuthServer == "" {
		cfg.Addr = "localhost:3025"
	} else if cfg.AuthServer != "" {
		log.Warn("Configuration setting `auth_server` is deprecated, consider to change it to `addr`")
	}

	if err := cfg.CheckTLSConfig(); err != nil {
		return trace.Wrap(err)
	}

	if cfg.Identity != "" && cfg.ClientCrt != "" {
		return trace.BadParameter("configuration setting `identity` is mutually exclusive with all the `client_crt`, `client_key` and `root_cas` settings")
	}

	return nil
}

func (cfg *TeleportConfig) CheckTLSConfig() error {
	provided := stringset.NewWithCap(3)
	missing := stringset.NewWithCap(3)

	if cfg.ClientCrt != "" {
		provided.Add("`client_crt`")
	} else {
		missing.Add("`client_crt`")
	}

	if cfg.ClientKey != "" {
		provided.Add("`client_key`")
	} else {
		missing.Add("`client_key`")
	}

	if cfg.RootCAs != "" {
		provided.Add("`root_cas`")
	} else {
		missing.Add("`root_cas`")
	}

	if len(provided) > 0 && len(provided) < 3 {
		return trace.BadParameter(
			"configuration setting(s) %s are provided but setting(s) %s are missing",
			strings.Join(provided.ToSlice(), ", "),
			strings.Join(missing.ToSlice(), ", "),
		)
	}

	return nil
}

func (cfg TeleportConfig) Credentials() []client.Credentials {
	switch true {
	case cfg.Identity != "":
		return []client.Credentials{client.LoadIdentityFile(cfg.Identity)}
	case cfg.ClientCrt != "" && cfg.ClientKey != "" && cfg.RootCAs != "":
		return []client.Credentials{client.LoadKeyPair(cfg.ClientCrt, cfg.ClientKey, cfg.RootCAs)}
	default:
		return nil
	}
}

// ReadPassword reads password from file or env var, trims and returns
func ReadPassword(filename string) (string, error) {
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		return "", trace.BadParameter("Error reading password from %v", filename)
	}
	if err != nil {
		return "", trace.Wrap(err)
	}
	pass := make([]byte, 2000)
	l, err := f.Read(pass)
	if err != nil && err != io.EOF {
		return "", err
	}
	pass = pass[:l] // truncate \0
	return strings.TrimSpace(string(pass)), nil
}

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

package opensearch

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

// ProfileName is the name of the opensearch-cli that will be created for Teleport usage
const ProfileName = "teleport"

// Certificate is an optional certificate config.
type Certificate struct {
	// CACert is the path to the CA cert.
	CACert string `json:"cafilepath,omitempty"`
	// Cert is the path to the client cert.
	Cert string `json:"clientcertificatefilepath,omitempty"`
	// Key is the path to the client key.
	Key string `json:"clientkeyfilepath,omitempty"`
}

// Profile represents single profile in opensearch-cli configuration
type Profile struct {
	// Name is the name of the profile. We use fixed "teleport" profile name per the ProfileName constant.
	Name string `json:"name"`
	// Endpoint is the URL of the database endpoint
	Endpoint string `json:"endpoint"`
	// Certificate holds optional certificate info
	Certificate *Certificate `json:"certificate,omitempty"`
	// MaxRetry is the maximum number of retries to be made in case of error.
	MaxRetry int `json:"max_retry,omitempty"`
	// Timeout is the timeout used by the client.
	Timeout int `json:"timeout,omitempty"`
}

// Config represents configuration for opensearch-cli
type Config struct {
	// Profiles is the list of profiles in the config.
	Profiles []Profile `json:"profiles"`
}

// ConfigNoTLS returns insecure config with single profile.
func ConfigNoTLS(host string, port int) Config {
	return Config{Profiles: []Profile{
		{
			Name:     ProfileName,
			Endpoint: fmt.Sprintf("http://%v:%v/", host, port),
			MaxRetry: 3,
			Timeout:  10,
		},
	}}
}

// ConfigTLS returns secure config with single profile.
func ConfigTLS(host string, port int, caCert, cert, key string) Config {
	return Config{Profiles: []Profile{
		{
			Name:     ProfileName,
			Endpoint: fmt.Sprintf("https://%v:%v/", host, port),
			Certificate: &Certificate{
				CACert: caCert,
				Cert:   cert,
				Key:    key,
			},
			MaxRetry: 3,
			Timeout:  10,
		},
	}}
}

// WriteConfig writes the config to disk, relative to the base dir.
func WriteConfig(baseDir string, cfg Config) (string, error) {
	// serialize config
	bytes, err := yaml.Marshal(cfg)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// calculate config hash
	h := fnv.New32()
	// h.Write() will never return an error.
	_, _ = h.Write(bytes)
	hash := h.Sum32()

	// create config directory if it doesn't exist
	configDir := filepath.Join(baseDir, "opensearch-cli")
	err = os.MkdirAll(configDir, 0700)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// write config to file
	fn := filepath.Join(configDir, fmt.Sprintf("%x.yml", hash))
	err = os.WriteFile(fn, bytes, 0600)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return fn, nil
}

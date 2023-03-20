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

package opensearch

import (
	"fmt"
	"hash/fnv"
	"os"
	"path"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

const ProfileName = "teleport"

type Certificate struct {
	CACert string `json:"cafilepath"`
	Cert   string `json:"clientcertificatefilepath"`
	Key    string `json:"clientkeyfilepath"`
}

type Profile struct {
	Name        string       `json:"name"`
	Endpoint    string       `json:"endpoint"`
	Certificate *Certificate `json:"certificate,omitempty"`
	MaxRetry    int          `json:"max_retry,omitempty"`
	Timeout     int          `json:"timeout,omitempty"`
}

type Config struct {
	Profiles []Profile `json:"profiles"`
}

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

func WriteTempConfig(baseDir string, cfg Config) (string, error) {
	// serialize config
	bytes, err := yaml.Marshal(cfg)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// calculate config hash
	h := fnv.New32()
	_, _ = h.Write(bytes)
	hash := h.Sum32()

	// create config directory if it doesn't exist
	configDir := path.Join(baseDir, "opensearch-cli")
	err = os.MkdirAll(configDir, 0700)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// write config to file
	fn := path.Join(configDir, fmt.Sprintf("%x.yml", hash))
	err = os.WriteFile(fn, bytes, 0600)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return fn, nil
}

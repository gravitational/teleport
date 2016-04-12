/*
Copyright 2015 Gravitational, Inc.

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

package etcdbk

import (
	"encoding/json"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// Config represents JSON config for etcd backend
type Config struct {
	Nodes       []string `json:"nodes"`
	Key         string   `json:"key"`
	TLSKeyFile  string   `json:"tls_key_file"`
	TLSCertFile string   `json:"tls_cert_file"`
	TLSCAFile   string   `json:"tls_ca_file"`
}

// Check checks if all the parameters are valid
func (cfg *Config) Check() error {
	if len(cfg.Key) == 0 {
		return trace.BadParameter(`Key: supply a valid root key for Teleport data`)
	}
	if len(cfg.Nodes) == 0 {
		return trace.BadParameter(`Nodes: please supply a valid dictionary, e.g. {"nodes": ["http://localhost:4001]}`)
	}
	if cfg.TLSKeyFile == "" {
		return trace.BadParameter(`TLSKeyFile: please supply a path to TLS private key file`)
	}
	if cfg.TLSCertFile == "" {
		return trace.BadParameter(`TLSCertFile: please supply a path to TLS certificate file`)
	}
	return nil
}

// FromObject initialized the backend from backend-specific string
func FromObject(in interface{}) (backend.Backend, error) {
	var cfg *Config
	if err := utils.ObjectToStruct(in, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return New(*cfg)
}

// FromJSON returns backend initialized from JSON-encoded string
func FromJSON(paramsJSON string) (backend.Backend, error) {
	cfg := Config{}
	err := json.Unmarshal([]byte(paramsJSON), &cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return New(cfg)
}

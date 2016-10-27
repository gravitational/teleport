// +build dynamodb

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

package dynamo

import (
	"encoding/json"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// Config represents JSON config for dynamodb backend
type Config struct {
	Tablename string `json:"tablename"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// ConfigureBackend configures DynamoDB backend
func ConfigureBackend(bconf *backend.Config) (string, error) {
	dynCfg := &Config{
		Region:    bconf.Region,
		AccessKey: bconf.AccessKey,
		SecretKey: bconf.SecretKey,
		Tablename: bconf.Tablename,
	}

	params, err := dynamodbParams(dynCfg)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return params, nil
}

// Check checks if all the parameters are valid
func (cfg *Config) Check() error {
	if len(cfg.Tablename) == 0 {
		return trace.BadParameter(`Tablename: supply the tablename where Teleport data are stored`)
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

// dynamodbParams generates a string accepted by the DynamoDB driver, like this:
func dynamodbParams(cfg *Config) (string, error) {
	out, err := json.Marshal(cfg)
	if err != nil { // don't know what to do seriously
		return "", trace.Wrap(err)
	}
	return string(out), nil
}

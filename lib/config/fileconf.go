/*
Copyright 2015-16 Gravitational, Inc.

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
package config

import (
	"gopkg.in/yaml.v2"
)

type AuthServer struct {
	Address string `yaml:"address"` // "tcp://127.0.0.1:3024"
	Token   string `yaml:"token"`   // "xxxxxxx"
}

type Storate struct {
	Type   string `yaml:"type"`
	Params string `yaml:"params"`
}

type ConnectionRate struct {
	Period  string `yaml:"period"`
	Average string `yaml:"average"`
	Burst   string `yaml:"burst"`
}

type ConnectionLimits struct {
	MaxConnections int              `yaml:"max_connections"`
	MaxUsers       int              `yaml:"max_users"`
	Rates          []ConnectionRate `yaml:"rates,omitempty"`
}

type Common struct {
	NodeName    string           `yaml:"nodename,omitempty"`
	AuthServers []AuthServer     `yaml:"auth_servers,omitempty"`
	CLimits     ConnectionLimits `yaml:"connection_limits,omitempty"`
}

// YAMLConfig represents configuration stored in a config file
// in YAML format (usually /etc/teleport.yaml)
type FileConfig struct {
	Common `yaml:"teleport,omitempty"`
}

func play() string {
	conf := FileConfig{}

	bytes, err := yaml.Marshal(&conf)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

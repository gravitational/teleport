/*
Copyright 2022 Gravitational, Inc.

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

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/api/profile"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
)

// .tsh config must go in a subdir as all .yaml files in .tsh get
// parsed automatically by the profile loader and results in yaml
// unmarshal errors.
const tshConfigPath = "config/config.yaml"

// default location of global tsh config file.
const globalTshConfigPathDefault = "/etc/tsh.yaml"

// TshConfig represents configuration loaded from the tsh config file.
type TshConfig struct {
	// ExtraHeaders are additional http headers to be included in
	// webclient requests.
	ExtraHeaders []ExtraProxyHeaders `yaml:"add_headers,omitempty"`
	// ProxyTemplates describe rules for parsing out proxy out of full hostnames.
	ProxyTemplates ProxyTemplates `yaml:"proxy_templates,omitempty"`
}

// Check validates the tsh config.
func (config *TshConfig) Check() error {
	for _, template := range config.ProxyTemplates {
		if err := template.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ExtraProxyHeaders represents the headers to include with the
// webclient.
type ExtraProxyHeaders struct {
	// Proxy is the domain of the proxy for these set of Headers, can contain globs.
	Proxy string `yaml:"proxy"`
	// Headers are the http header key values.
	Headers map[string]string `yaml:"headers,omitempty"`
}

// Merge two configs into one. The passed in otherConfig argument has higher priority.
func (config *TshConfig) Merge(otherConfig *TshConfig) TshConfig {
	baseConfig := config
	if baseConfig == nil {
		baseConfig = &TshConfig{}
	}

	if otherConfig == nil {
		otherConfig = &TshConfig{}
	}

	newConfig := TshConfig{}
	newConfig.ExtraHeaders = append(otherConfig.ExtraHeaders, baseConfig.ExtraHeaders...)
	newConfig.ProxyTemplates = append(otherConfig.ProxyTemplates, baseConfig.ProxyTemplates...)

	return newConfig
}

// ProxyTemplates represents a list of individual proxy templates.
type ProxyTemplates []*ProxyTemplate

// Apply attempts to match the provided full hostname against all the templates
// in the list. Returns extracted proxy and host upon encountering the first
// matching template.
func (t ProxyTemplates) Apply(fullHostname string) (proxy, host string, matched bool) {
	for _, template := range t {
		proxy, host, matched := template.Apply(fullHostname)
		if matched {
			return proxy, host, true
		}
	}
	return "", "", false
}

// ProxyTemplate describes a single rule for parsing out proxy address from
// the full hostname. Used by tsh proxy ssh.
type ProxyTemplate struct {
	// Template is a regular expression that full hostname is matched against.
	Template string `yaml:"template"`
	// Proxy is the proxy address. Can refer to regex groups from the template.
	Proxy string `yaml:"proxy"`
	// Host is optional hostname. Can refer to regex groups from the template.
	Host string `yaml:"host"`
	// re is the compiled template regexp.
	re *regexp.Regexp
}

// Check validates the proxy template.
func (t *ProxyTemplate) Check() (err error) {
	if strings.TrimSpace(t.Proxy) == "" {
		return trace.BadParameter("empty proxy expression")
	}
	if strings.TrimSpace(t.Template) == "" {
		return trace.BadParameter("empty proxy template")
	}
	t.re, err = regexp.Compile(t.Template)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Apply applies the proxy template to the provided hostname and returns
// expanded proxy address and hostname.
func (t ProxyTemplate) Apply(fullHostname string) (proxy, host string, matched bool) {
	match := t.re.FindAllStringSubmatchIndex(fullHostname, -1)
	if match == nil {
		return "", "", false
	}

	expandedProxy := []byte{}
	for _, m := range match {
		expandedProxy = t.re.ExpandString(expandedProxy, t.Proxy, fullHostname, m)
	}
	proxy = string(expandedProxy)

	host = fullHostname
	if t.Host != "" {
		expandedHost := []byte{}
		for _, m := range match {
			expandedHost = t.re.ExpandString(expandedHost, t.Host, fullHostname, m)
		}
		host = string(expandedHost)
	}

	return proxy, host, true
}

// loadConfig load a single config file from given path. If the path does not exist, an empty config is returned instead.
func loadConfig(fullConfigPath string) (*TshConfig, error) {
	bs, err := os.ReadFile(fullConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &TshConfig{}, nil
		}
		return nil, trace.ConvertSystemError(err)
	}
	cfg := TshConfig{}
	if err := yaml.Unmarshal(bs, &cfg); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
}

// loadAllConfigs loads all tsh configs and merges them in appropriate order.
func loadAllConfigs(cf CLIConf) (*TshConfig, error) {
	// default to globalTshConfigPathDefault
	globalConfigPath := cf.GlobalTshConfigPath
	if globalConfigPath == "" {
		globalConfigPath = globalTshConfigPathDefault
	}

	globalConf, err := loadConfig(globalConfigPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to load global tsh config from %q", cf.GlobalTshConfigPath)
	}

	fullConfigPath := filepath.Join(profile.FullProfilePath(cf.HomePath), tshConfigPath)
	userConf, err := loadConfig(fullConfigPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to load tsh config from %q", fullConfigPath)
	}

	confOptions := globalConf.Merge(userConf)
	return &confOptions, nil
}

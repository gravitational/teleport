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

package jira

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"
	"strings"

	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/trace"
)

type Config struct {
	Teleport lib.TeleportConfig `toml:"teleport"`
	Jira     JiraConfig         `toml:"jira"`
	HTTP     lib.HTTPConfig     `toml:"http"`
	Log      logger.Config      `toml:"log"`
}

type JiraConfig struct {
	URL       string `toml:"url"`
	Username  string `toml:"username"`
	APIToken  string `toml:"api_token"`
	Project   string `toml:"project"`
	IssueType string `toml:"issue_type"`
}

func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Jira.URL == "" {
		return trace.BadParameter("missing required value jira.url")
	}
	if !strings.HasPrefix(c.Jira.URL, "https://") {
		return trace.BadParameter("parameter jira.url must start with \"https://\"")
	}
	if c.Jira.Username == "" {
		return trace.BadParameter("missing required value jira.username")
	}
	if c.Jira.APIToken == "" {
		return trace.BadParameter("missing required value jira.api_token")
	}
	if c.Jira.IssueType == "" {
		c.Jira.IssueType = "Task"
	}
	if c.HTTP.ListenAddr == "" {
		c.HTTP.ListenAddr = ":8081"
	}
	if err := c.HTTP.Check(); err != nil {
		return trace.Wrap(err)
	}
	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}
	return nil
}

// LoadTLSConfig loads client crt/key files and root authorities, and
// generates a tls.Config suitable for use with a GRPC client.
func (c *Config) LoadTLSConfig() (*tls.Config, error) {
	var tc tls.Config
	clientCert, err := tls.LoadX509KeyPair(c.Teleport.ClientCrt, c.Teleport.ClientKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.Certificates = append(tc.Certificates, clientCert)
	caFile, err := os.Open(c.Teleport.RootCAs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCerts, err := io.ReadAll(caFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCerts); !ok {
		return nil, trace.BadParameter("invalid CA cert PEM")
	}
	tc.RootCAs = pool
	return &tc, nil
}

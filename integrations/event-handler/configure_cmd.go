/*
Copyright 2015-2021 Gravitational, Inc.

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
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/event-handler/lib"
)

// ConfigureCmd represents configure command behavior.
//
// teleport-event-handler configure .
//
// It generates fluentd mTLS certificates, example configuration and teleport user/role definitions.
//
// mTLS certificates are self-signed by our own generated CA. So, we generate three certificates: CA, server
// which is used on the fluentd side, and client which is used by the plugin to connect to a fluentd instance.
//
// Please, check README.md for additional info.
type ConfigureCmd struct {
	*ConfigureCmdConfig

	// step holds step number for cli messages
	step int

	// caCertPath is a path to a target fluentd mTLS CA cert file
	caCertPath string

	// caKeyPath is a path to a target fluentd mTLS private key file
	caKeyPath string

	// serverCertPath is a path to a target mTLS server cert file used by a fluentd instance
	serverCertPath string

	// serverKeyPath is a path to a target mTLS server private key file used by a fluentd instance
	serverKeyPath string

	// clientCertPath is a path to a target mTLS client cert file used by a plugin's fluentd client
	clientCertPath string

	// clientKeyPath is a path to a target mTLS client private key file used by a plugin's fluentd client
	clientKeyPath string

	// roleDefPath path to target role definition file which contains plugin role and user
	roleDefPath string

	// fluentdConfPath path to target fluentd configuration file which contains an example fluentd configuration
	fluentdConfPath string

	// confPath path to target plugin configuration file which contains an example plugin configuration
	confPath string

	// mtls is the struct with generated mTLS certificates
	mtls *MTLSCerts
}

var (
	// maxBigInt is serial number random max
	maxBigInt = new(big.Int).Lsh(big.NewInt(1), 128)

	//go:embed tpl/teleport-event-handler-role.yaml.tpl
	roleTpl string

	//go:embed tpl/teleport-event-handler.toml.tpl
	confTpl string

	//go:embed tpl/fluent.conf.tpl
	fluentdConfTpl string
)

const (
	// perms certificate/key file permissions
	perms = 0600

	// passwordLength represents rand password length
	passwordLength = 32

	// roleDefFileName is role definition file name
	roleDefFileName = "teleport-event-handler-role.yaml"

	// fluentdConfFileName is fluentd config file name
	fluentdConfFileName = "fluent.conf"

	// confFileName is plugin configuration file name
	confFileName = "teleport-event-handler.toml"

	// guideURL is getting started guide URL
	guideURL = "https://goteleport.com/docs/management/export-audit-events/fluentd/"
)

// RunConfigureCmd initializes and runs configure command
func RunConfigureCmd(cfg *ConfigureCmdConfig) error {
	c := ConfigureCmd{
		ConfigureCmdConfig: cfg,
		caCertPath:         path.Join(cfg.Out, cfg.CAName) + ".crt",
		caKeyPath:          path.Join(cfg.Out, cfg.CAName) + ".key",
		serverCertPath:     path.Join(cfg.Out, cfg.ServerName) + ".crt",
		serverKeyPath:      path.Join(cfg.Out, cfg.ServerName) + ".key",
		clientCertPath:     path.Join(cfg.Out, cfg.ClientName) + ".crt",
		clientKeyPath:      path.Join(cfg.Out, cfg.ClientName) + ".key",
		roleDefPath:        path.Join(cfg.Out, roleDefFileName),
		fluentdConfPath:    path.Join(cfg.Out, fluentdConfFileName),
		confPath:           path.Join(cfg.Out, confFileName),
	}

	g, err := GenerateMTLSCerts(cfg.DNSNames, cfg.IP, cfg.TTL, cfg.Length)
	if err != nil {
		return trace.Wrap(err)
	}

	c.mtls = g

	return c.Run()
}

// Run runs the generator
func (c *ConfigureCmd) Run() error {
	fmt.Printf("Teleport event handler %v %v\n\n", Version, Gitref)

	c.step = 1

	// Get password either from STDIN or generated string
	pwd, err := c.getPwd()
	if err != nil {
		return trace.Wrap(err)
	}

	// Generate certificates and save them to desired locations
	err = c.genCerts(pwd)
	if err != nil {
		return trace.Wrap(err)
	}

	// Print paths to generated fluentd certificate files
	paths := []string{c.caCertPath, c.caKeyPath, c.serverCertPath, c.serverKeyPath, c.clientCertPath, c.clientKeyPath}
	paths, err = c.cleanupPaths(paths...)
	if err != nil {
		return trace.Wrap(err)
	}

	c.printStep("Generated mTLS Fluentd certificates %v", strings.Join(paths, ", "))

	// Write role definition file
	err = c.writeRoleDef()
	if err != nil {
		return trace.Wrap(err)
	}

	path, err := c.cleanupPath(c.roleDefPath)
	if err != nil {
		return trace.Wrap(err)
	}

	c.printStep("Generated sample teleport-event-handler role and user file %v", path)

	// Write fluentd configuration file
	err = c.writeFluentdConf(pwd)
	if err != nil {
		return trace.Wrap(err)
	}

	path, err = c.cleanupPath(c.fluentdConfPath)
	if err != nil {
		return trace.Wrap(err)
	}

	c.printStep("Generated sample fluentd configuration file %v", path)

	// Write main configuration file
	err = c.writeConf()
	if err != nil {
		return trace.Wrap(err)
	}

	path, err = c.cleanupPath(c.confPath)
	if err != nil {
		return trace.Wrap(err)
	}

	c.printStep("Generated plugin configuration file %v", path)

	fmt.Println()
	fmt.Println("Follow-along with our getting started guide:")
	fmt.Println()
	fmt.Println(guideURL)

	return nil
}

// cleanupPaths cleans up paths passed as arguments
func (c *ConfigureCmd) cleanupPaths(args ...string) ([]string, error) {
	result := make([]string, len(args))

	rel, err := os.Getwd()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for i, p := range args {
		r, err := filepath.Rel(rel, p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		r = filepath.Clean(r)

		if strings.Contains(r, "..") {
			r, err = filepath.Abs(r)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		result[i] = r
	}

	return result, nil
}

// cleanupPath cleans up a single path
func (c *ConfigureCmd) cleanupPath(arg string) (string, error) {
	p, err := c.cleanupPaths(arg)
	if err != nil {
		return "", err
	}

	return p[0], nil
}

// Generates fluentd certificates
func (c *ConfigureCmd) genCerts(pwd string) error {
	ok := c.askOverwrite(c.caKeyPath)
	if ok {
		err := c.mtls.CACert.WriteFile(c.caCertPath, c.caKeyPath, "")
		if err != nil {
			return trace.Wrap(err)
		}
	}

	ok = c.askOverwrite(c.serverKeyPath)
	if ok {
		err := c.mtls.ServerCert.WriteFile(c.serverCertPath, c.serverKeyPath, pwd)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	ok = c.askOverwrite(c.clientKeyPath)
	if ok {
		err := c.mtls.ClientCert.WriteFile(c.clientCertPath, c.clientKeyPath, "")
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// getPwd returns password read from STDIN or generates new if no password is provided
func (c *ConfigureCmd) getPwd() (string, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Get password from provided file
		pwdFromStdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}

		return string(pwdFromStdin), nil
	}

	// Otherwise, generate random hex token
	bytes := make([]byte, passwordLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

// writePwd writes generated password to the file
func (c *ConfigureCmd) writeFile(path string, content []byte) error {
	ok := c.askOverwrite(path)
	if !ok {
		return nil
	}

	err := os.WriteFile(path, content, perms)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// printStep prints step with number
func (c *ConfigureCmd) printStep(message string, args ...interface{}) {
	p := append([]interface{}{c.step}, args...)
	fmt.Printf("[%v] "+message+"\n", p...)
	c.step++
}

// writeRoleDef writes role definition file
func (c *ConfigureCmd) writeRoleDef() error {
	var b bytes.Buffer

	err := lib.RenderTemplate(roleTpl, nil, &b)
	if err != nil {
		return trace.Wrap(err)
	}

	return c.writeFile(c.roleDefPath, b.Bytes())
}

// writeFluentdConf writes fluentd config file
func (c *ConfigureCmd) writeFluentdConf(pwd string) error {
	var b bytes.Buffer
	var pipeline = struct {
		CaCertFileName     string
		ServerCertFileName string
		ServerKeyFileName  string
		Pwd                string
	}{
		path.Base(c.caCertPath),
		path.Base(c.serverCertPath),
		path.Base(c.serverKeyPath),
		pwd,
	}

	err := lib.RenderTemplate(fluentdConfTpl, pipeline, &b)
	if err != nil {
		return trace.Wrap(err)
	}

	return c.writeFile(c.fluentdConfPath, b.Bytes())
}

// writeFluentdConf writes fluentd config file
func (c *ConfigureCmd) writeConf() error {
	var b bytes.Buffer
	var pipeline = struct {
		CaCertPath     string
		ClientCertPath string
		ClientKeyPath  string
		Addr           string
	}{c.caCertPath, c.clientCertPath, c.clientKeyPath, c.Addr}

	err := lib.RenderTemplate(confTpl, pipeline, &b)
	if err != nil {
		return trace.Wrap(err)
	}

	return c.writeFile(c.confPath, b.Bytes())
}

// askOverwrite asks question if the user wants to overwrite specified file if it exists
func (c *ConfigureCmd) askOverwrite(path string) bool {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return lib.AskYesNo(fmt.Sprintf("Do you want to overwrite %s", path))
	}

	return true
}

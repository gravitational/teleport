// Copyright 2022 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package kinit provides utilities for interacting with a KDC (Key Distribution Center) for Kerberos5
package kinit

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/windows"
)

const (
	// krb5ConfigEnv sets the location from which kinit will attempt to read a configuration value
	krb5ConfigEnv = "KRB5_CONFIG"
	// kinitBinary is the binary Name for the kinit executable
	kinitBinary = "kinit"
	// krb5ConfigTemplate is a configuration template suitable for x509 configuration; it is read by the kinit binary
	krb5ConfigTemplate = `[libdefaults]
 default_realm = {{ .RealmName }}
 rdns = false


[realms]
 {{ .RealmName }} = {
  kdc = {{ .KDCHostName }}
  admin_server = {{ .AdminServerName }}
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = {{ .KDCHostName }}
 }`
	// certTTL is the certificate time to live; 1 hour
	certTTL = time.Minute * 60
)

// Provider is a kinit provider capable of producing a credentials cacheData for kerberos
type Provider interface {
	// UseOrCreateCredentials uses or updates an existing cacheData or creates a new one
	UseOrCreateCredentials(ctx context.Context) (cache *credentials.CCache, err error)
}

// PKInit is a structure used for initializing a kerberos context
type PKInit struct {
	provider Provider
}

// UseOrCreateCredentialsCache uses or creates a credentials cacheData.
func (k *PKInit) UseOrCreateCredentialsCache(ctx context.Context) (*credentials.CCache, error) {
	return k.provider.UseOrCreateCredentials(ctx)
}

// New returns a new PKInit initializer
func New(provider Provider) *PKInit {
	return &PKInit{provider: provider}
}

// CommandConfig is used to configure a kinit binary execution
type CommandConfig struct {
	// AuthClient is a subset of the auth interface
	AuthClient windows.AuthInterface
	// User is the username of the database/AD user
	User string
	// Realm is the domain name
	Realm string
	// KDCHost is the key distribution center hostname (usually AD server)
	KDCHost string
	// AdminServer is the administration server hostname (usually AD server)
	AdminServer string
	// DataDir is the Teleport Data Directory
	DataDir string
	// LDAPCA is the Windows LDAP Certificate for client signing
	LDAPCA *x509.Certificate
	// Command is a command generator that generates an executable command
	Command CommandGenerator
	// CertGetter is a Teleport Certificate getter that prepares an x509 certificate
	// for use with windows AD
	CertGetter CertGetter
}

// NewCommandLineInitializer returns a new command line initializer using a preinstalled `kinit` binary
func NewCommandLineInitializer(config CommandConfig) *CommandLineInitializer {
	cmd := &CommandLineInitializer{
		auth:            config.AuthClient,
		userName:        config.User,
		cacheName:       fmt.Sprintf("%s@%s", config.User, config.Realm),
		RealmName:       config.Realm,
		KDCHostName:     config.KDCHost,
		AdminServerName: config.AdminServer,
		dataDir:         config.DataDir,
		certPath:        fmt.Sprintf("%s.pem", config.User),
		keyPath:         fmt.Sprintf("%s-key.pem", config.User),
		binary:          kinitBinary,
		command:         config.Command,
		certGetter:      config.CertGetter,
		ldapCertificate: config.LDAPCA,
		log:             logrus.StandardLogger(),
	}
	if cmd.command == nil {
		cmd.command = &execCmd{}
	}
	return cmd
}

// CommandGenerator is a small interface for wrapping *exec.Cmd
type CommandGenerator interface {
	// CommandContext is a wrapper for creating a command
	CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd
}

// execCmd is a small wrapper around exec.Cmd
type execCmd struct {
}

// CommandContext returns exec.CommandContext
func (e *execCmd) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// CommandLineInitializer uses a command line `kinit` binary to provide a kerberos CCache
type CommandLineInitializer struct {
	auth windows.AuthInterface

	// RealmName is the kerberos realm Name (domain Name, like `example.com`
	RealmName string
	// KDCHostName is the key distribution center host Name (usually AD host, like ad.example.com)
	KDCHostName string
	// AdminServerName is the admin server Name (usually AD host)
	AdminServerName string

	dataDir   string
	userName  string
	cacheName string

	certPath string
	keyPath  string
	binary   string

	command    CommandGenerator
	certGetter CertGetter

	ldapCertificate *x509.Certificate
	log             logrus.FieldLogger
}

// CertGetter is an interface for getting a new cert/key pair along with a CA cert
type CertGetter interface {
	// GetCertificateBytes returns a new cert/key pair along with a CA for use with x509 Auth
	GetCertificateBytes(ctx context.Context) (*WindowsCAAndKeyPair, error)
}

// DBCertGetter obtains a new cert/key pair along with the Teleport database CA
type DBCertGetter struct {
	// Auth is the auth client
	Auth windows.AuthInterface
	// KDCHostName is the Name of the key distribution center host
	KDCHostName string
	// RealmName is the kerberos realm Name (domain Name)
	RealmName string
	// AdminServerName is the Name of the admin server. Usually same as the KDC
	AdminServerName string
	// UserName is the database username
	UserName string
	// LDAPCA is the windows ldap certificate
	LDAPCA *x509.Certificate
	// CAFunc returns a TLSKeyPair of certificate bytes
	CAFunc func(ctx context.Context, clusterName string) ([]byte, error)
}

func (d *DBCertGetter) caFunc(ctx context.Context, clusterName string) ([]byte, error) {

	dbCA, err := d.Auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var caCert []byte
	keyPairs := dbCA.GetActiveKeys().TLS
	for _, keyPair := range keyPairs {
		if keyPair.KeyType == types.PrivateKeyType_RAW {
			caCert = keyPair.Cert
		}
	}
	return caCert, nil
}

// WindowsCAAndKeyPair is a wrapper around PEM bytes for Windows authentication
type WindowsCAAndKeyPair struct {
	certPEM []byte
	keyPEM  []byte
	caCert  []byte
}

// GetCertificateBytes returns a new cert/key pem and the DB CA bytes
func (d *DBCertGetter) GetCertificateBytes(ctx context.Context) (*WindowsCAAndKeyPair, error) {
	clusterName, err := d.Auth.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certPEM, keyPEM, err := windows.CertKeyPEM(ctx, &windows.GenerateCredentialsRequest{
		Username:    d.UserName,
		Domain:      d.RealmName,
		TTL:         certTTL,
		ClusterName: clusterName.GetClusterName(),
		LDAPConfig: windows.LDAPConfig{
			Addr:               d.KDCHostName,
			Domain:             d.RealmName,
			Username:           d.UserName,
			InsecureSkipVerify: false,
			ServerName:         d.AdminServerName,
			CA:                 d.LDAPCA,
		},
		AuthClient: d.Auth,
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	if d.CAFunc == nil {
		d.CAFunc = d.caFunc
	}

	caCert, err := d.CAFunc(ctx, clusterName.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if caCert == nil {
		return nil, trace.BadParameter("no certificate authority was found in userCA active keys")
	}

	return &WindowsCAAndKeyPair{certPEM: certPEM, keyPEM: keyPEM, caCert: caCert}, nil
}

// UseOrCreateCredentials uses an existing cacheData or creates a new one
func (k *CommandLineInitializer) UseOrCreateCredentials(ctx context.Context) (*credentials.CCache, error) {
	tmp, err := os.MkdirTemp("", "kinit")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		err = os.RemoveAll(tmp)
		if err != nil {
			k.log.Errorf("failed removing temporary kinit directory: %s", err)
		}
	}()

	certPath := filepath.Join(tmp, fmt.Sprintf("%s.pem", k.userName))
	keyPath := filepath.Join(tmp, fmt.Sprintf("%s-key.pem", k.userName))
	userCAPath := filepath.Join(tmp, "userca.pem")

	cacheDir := filepath.Join(k.dataDir, "krb5_cache")

	err = os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cachePath := filepath.Join(cacheDir, k.cacheName)

	wca, err := k.certGetter.GetCertificateBytes(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// store files in temp dir
	err = os.WriteFile(certPath, wca.certPEM, 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = os.WriteFile(keyPath, wca.keyPEM, 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = os.WriteFile(userCAPath, wca.caCert, 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	krbConfPath := filepath.Join(tmp, fmt.Sprintf("krb_%s", k.userName))
	err = k.WriteKRB5Config(krbConfPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmd := k.command.CommandContext(ctx,
		k.binary,
		"-X", fmt.Sprintf("X509_anchors=FILE:%s", certPath),
		"-X", fmt.Sprintf("X509_user_identity=FILE:%s,%s", certPath, keyPath), k.userName,
		"-c", cachePath)

	if cmd.Err != nil {
		return nil, trace.Wrap(cmd.Err)
	}

	cmd.Env = append(cmd.Env, []string{fmt.Sprintf("%s=%s", krb5ConfigEnv, krbConfPath)}...)
	err = cmd.Run()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ccache, err := credentials.LoadCCache(cachePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ccache, nil
}

// krb5ConfigString returns a config suitable for a kdc
func (k *CommandLineInitializer) krb5ConfigString() (string, error) {
	t, err := template.New("krb_conf").Parse(krb5ConfigTemplate)

	if err != nil {
		return "", trace.Wrap(err)
	}
	b := bytes.NewBuffer([]byte{})
	err = t.Execute(b, k)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return b.String(), nil
}

// WriteKRB5Config writes a krb configuration to path
func (k *CommandLineInitializer) WriteKRB5Config(path string) error {
	s, err := k.krb5ConfigString()
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.WriteFile(path, []byte(s), 0644))
}

/*
Copyright 2023 Gravitational, Inc.

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

package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	// agentlessKeys is the path to write agentless openssh keys
	agentlessKeysDir = "/etc/teleport/agentless"
	// agentlessKeys is the path to write agentless openssh keys for
	// use on rollback
	agentlessKeysBackupDir = "/etc/teleport/agentless_backup"
)

const (
	openSSHRotateRollback = "rollback"
	openSSHRotateUpdate   = "update"
)

const (
	teleportKey       = "teleport"
	teleportCert      = "teleport-cert.pub"
	teleportOpenSSHCA = "teleport_user_ca.pub"
)

// GenerateKeys generates TLS and SSH keypairs.
func GenerateKeys() (privateKey, publicKey, tlsPublicKey []byte, err error) {
	privateKey, publicKey, err = native.GenerateKeyPair()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	tlsPublicKey, err = tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	return privateKey, publicKey, tlsPublicKey, nil
}

func ReadKeys(clf config.CommandLineFlags) (privateKey, publicKey, tlsPublicKey []byte, err error) {
	pkeyContents, err := os.ReadFile(filepath.Join(clf.OpenSSHKeysPath, teleportKey))
	if err != nil {
		return nil, nil, nil, trace.ConvertSystemError(err)
	}

	pkey, err := keys.ParsePrivateKey(pkeyContents)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	privateKey = pkey.PrivateKeyPEM()
	publicKey = pkey.MarshalSSHPublicKey()

	sshPrivateKey, err := ssh.ParseRawPrivateKey(pkeyContents)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	tlsPublicKey, err = tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return privateKey, publicKey, tlsPublicKey, nil
}

func authenticatedUserClientFromIdentity(ctx context.Context, insecure, fips bool, proxy utils.NetAddr, id *auth.Identity) (auth.ClientI, error) {
	var tlsConfig *tls.Config
	var err error
	var cipherSuites []uint16
	if fips {
		cipherSuites = defaults.FIPSCipherSuites
	}
	tlsConfig, err = id.TLSConfig(cipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.InsecureSkipVerify = insecure

	sshConfig, err := id.SSHClientConfig(fips)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authClientConfig := &authclient.Config{
		TLS:         tlsConfig,
		SSH:         sshConfig,
		AuthServers: []utils.NetAddr{proxy},
		Log:         log.StandardLogger(),
	}

	c, err := authclient.Connect(ctx, authClientConfig)
	return c, trace.Wrap(err)
}

func getAWSInstanceHostname(ctx context.Context, imds agentlessIMDS) (string, error) {
	hostname, err := imds.GetHostname(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	hostname = strings.ReplaceAll(hostname, " ", "_")
	if utils.IsValidHostname(hostname) {
		return hostname, nil
	}
	return "", trace.NotFound("failed to get a valid hostname from IMDS")
}

func tryCreateDefaultAgentlesKeysDir(agentlessKeysPath string) error {
	baseTeleportDir := filepath.Dir(agentlessKeysPath)
	_, err := os.Stat(baseTeleportDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debugf("%s did not exist, creating %s", baseTeleportDir, agentlessKeysPath)
			return trace.Wrap(os.MkdirAll(agentlessKeysPath, 0700))
		}
		return trace.Wrap(err)
	}

	var alreadyExistedAndDeleted bool
	_, err = os.Stat(agentlessKeysPath)
	if err == nil {
		log.Debugf("%s already existed, removing old files", agentlessKeysPath)
		err = os.RemoveAll(agentlessKeysPath)
		if err != nil {
			return trace.Wrap(err)
		}
		alreadyExistedAndDeleted = true
	}

	if os.IsNotExist(err) || alreadyExistedAndDeleted {
		log.Debugf("%s did not exist, creating", agentlessKeysPath)
		return trace.Wrap(os.Mkdir(agentlessKeysPath, 0700))
	}

	return trace.Wrap(err)
}

func getEC2ID(ctx context.Context, imds agentlessIMDS) (string, error) {
	accountID, err := imds.GetAccountID(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	instanceID, err := imds.GetID(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return fmt.Sprintf("%s-%s", accountID, instanceID), nil
}

type agentless struct {
	uuid                 string
	principals           []string
	hostname             string
	addr                 *utils.NetAddr
	imds                 agentlessIMDS
	defaultKeysDir       string
	defaultBackupKeysDir string
	restartSSHD          func() error
	clock                clockwork.Clock
}

type agentlessIMDS interface {
	GetHostname(context.Context) (string, error)
	GetAccountID(context.Context) (string, error)
	GetID(context.Context) (string, error)
}

func newAgentless(ctx context.Context, clf config.CommandLineFlags) (*agentless, error) {
	addr, err := utils.ParseAddr(clf.ProxyServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	imds, err := aws.NewInstanceMetadataClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostname, err := getAWSInstanceHostname(ctx, imds)
	if err != nil {
		var hostErr error
		hostname, hostErr = os.Hostname()
		if hostErr != nil {
			return nil, trace.NewAggregate(err, hostErr)
		}
	}

	nodeUUID, err := getEC2ID(ctx, imds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	principals := []string{nodeUUID}
	for _, principal := range strings.Split(clf.AdditionalPrincipals, ",") {
		if principal == "" {
			continue
		}
		principals = append(principals, principal)
	}

	return &agentless{
		uuid:                 nodeUUID,
		principals:           principals,
		hostname:             hostname,
		addr:                 addr,
		imds:                 imds,
		defaultKeysDir:       agentlessKeysDir,
		defaultBackupKeysDir: agentlessKeysBackupDir,
		restartSSHD:          restartSSHD,
	}, nil
}

func (a *agentless) register(clf config.CommandLineFlags, sshPublicKey, tlsPublicKey []byte) (*proto.Certs, error) {
	registerParams := auth.RegisterParams{
		Token:                clf.AuthToken,
		AdditionalPrincipals: a.principals,
		JoinMethod:           types.JoinMethod(clf.JoinMethod),
		ID: auth.IdentityID{
			Role:     types.RoleNode,
			NodeName: a.hostname,
			HostUUID: a.uuid,
		},
		AuthServers:        []utils.NetAddr{*a.addr},
		PublicTLSKey:       tlsPublicKey,
		PublicSSHKey:       sshPublicKey,
		GetHostCredentials: client.HostCredentials,
		FIPS:               clf.FIPS,
		CAPins:             clf.CAPins,
		Clock:              a.clock,
	}

	if clf.FIPS {
		registerParams.CipherSuites = defaults.FIPSCipherSuites
	}

	certs, err := auth.Register(registerParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, trace.Wrap(err)
}

func (a *agentless) getOpenSSHCA(ctx context.Context, insecure, fips bool, privateKey []byte, certs *proto.Certs) ([]byte, error) {
	identity, err := auth.ReadIdentityFromKeyPair(privateKey, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := authenticatedUserClientFromIdentity(ctx, insecure, fips, *a.addr, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer client.Close()

	cas, err := client.GetCertAuthorities(ctx, types.OpenSSHCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var openSSHCA []byte
	for _, ca := range cas {
		for _, key := range ca.GetActiveKeys().SSH {
			openSSHCA = append(openSSHCA, key.PublicKey...)
			openSSHCA = append(openSSHCA, byte('\n'))
		}
	}

	return openSSHCA, nil
}

func (a *agentless) openSSHInitialJoin(ctx context.Context, clf config.CommandLineFlags) error {
	if err := checkSSHDConfigAlreadyUpdated(clf.OpenSSHConfigPath); err != nil {
		return trace.Wrap(err)
	}

	privateKey, sshPublicKey, tlsPublicKey, err := GenerateKeys()
	if err != nil {
		return trace.Wrap(err, "unable to generate new keypairs")
	}

	certs, err := a.register(clf, sshPublicKey, tlsPublicKey)
	if err != nil {
		return trace.Wrap(err)
	}

	defaultKeysPath := clf.OpenSSHKeysPath == a.defaultKeysDir
	if defaultKeysPath {
		if err := tryCreateDefaultAgentlesKeysDir(a.defaultKeysDir); err != nil {
			return trace.Wrap(err)
		}
	}

	openSSHCA, err := a.getOpenSSHCA(ctx, clf.InsecureMode, clf.FIPS, privateKey, certs)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Writing Teleport keys to %s\n", clf.OpenSSHKeysPath)
	if err := writeKeys(clf.OpenSSHKeysPath, privateKey, certs, openSSHCA); err != nil {
		if defaultKeysPath {
			rmdirErr := os.RemoveAll(a.defaultKeysDir)
			if rmdirErr != nil {
				return trace.NewAggregate(err, rmdirErr)
			}
		}
		return trace.Wrap(err)
	}

	fmt.Println("Updating OpenSSH config")
	if err := updateSSHDConfig(clf.OpenSSHKeysPath, clf.OpenSSHConfigPath); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Restarting the OpenSSH daemon")
	if err := a.restartSSHD(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *agentless) openSSHRotateStageUpdate(ctx context.Context, clf config.CommandLineFlags) error {
	priv, pub, tlspub, err := ReadKeys(clf)
	if err != nil {
		// if the keys are not found do an initial join
		if trace.IsNotFound(err) {
			log.Debug("no keys found,attempting join from scratch")
			return trace.Wrap(a.openSSHInitialJoin(ctx, clf))
		}
		return trace.Wrap(err)
	}

	certs, err := a.register(clf, pub, tlspub)
	if err != nil {
		return trace.Wrap(err)
	}

	openSSHCA, err := a.getOpenSSHCA(ctx, clf.InsecureMode, clf.FIPS, priv, certs)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := os.RemoveAll(clf.OpenSSHKeysBackupPath); err != nil {
		return trace.Wrap(err)
	}

	// move the old keys to the backup directory
	if err := os.Rename(clf.OpenSSHKeysPath, clf.OpenSSHKeysBackupPath); err != nil {
		return trace.Wrap(err)
	}

	if err := tryCreateDefaultAgentlesKeysDir(clf.OpenSSHKeysPath); err != nil {
		return trace.Wrap(err)
	}

	if err := writeKeys(clf.OpenSSHKeysPath, priv, certs, openSSHCA); err != nil {
		return trace.Wrap(err)
	}

	if err := a.restartSSHD(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *agentless) openSSHRotateStageRollback(clf config.CommandLineFlags) error {
	if err := os.RemoveAll(clf.OpenSSHKeysPath); err != nil {
		return trace.Wrap(err)
	}

	// move the old keys to the backup directory
	if err := os.Rename(clf.OpenSSHKeysBackupPath, clf.OpenSSHKeysPath); err != nil {
		return trace.Wrap(err)
	}

	if err := a.restartSSHD(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeKeys(sshdConfigDir string, private []byte, certs *proto.Certs, openSSHCA []byte) error {
	if err := os.WriteFile(filepath.Join(sshdConfigDir, teleportKey), private, 0600); err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(filepath.Join(sshdConfigDir, teleportCert), certs.SSH, 0600); err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(filepath.Join(sshdConfigDir, teleportOpenSSHCA), openSSHCA, 0600); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

const sshdConfigSectionModificationHeader = "### Section created by 'teleport join openssh'"

func checkSSHDConfigAlreadyUpdated(sshdConfigPath string) error {
	contents, err := os.ReadFile(sshdConfigPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if strings.Contains(string(contents), sshdConfigSectionModificationHeader) {
		return trace.AlreadyExists("not updating %s as it has already been modified by teleport", sshdConfigPath)
	}
	return nil
}

const sshdBinary = "sshd"

func updateSSHDConfig(keyDir, sshdConfigPath string) error {
	// has to write to the beginning of the sshd_config file as
	// openssh takes the first occurrence of a setting
	sshdConfig, err := os.OpenFile(sshdConfigPath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return trace.Wrap(err)
	}
	defer sshdConfig.Close()

	configUpdate := fmt.Sprintf(`
%s
TrustedUserCaKeys %s
HostKey %s
HostCertificate %s
### Section end
`,
		sshdConfigSectionModificationHeader,
		filepath.Join(keyDir, teleportOpenSSHCA),
		filepath.Join(keyDir, teleportKey),
		filepath.Join(keyDir, teleportCert),
	)
	sshdConfigTmp, err := os.CreateTemp(keyDir, "")
	if err != nil {
		return trace.Wrap(err)
	}
	defer sshdConfigTmp.Close()
	if _, err := sshdConfigTmp.Write([]byte(configUpdate)); err != nil {
		return trace.Wrap(err)
	}

	if _, err := io.Copy(sshdConfigTmp, sshdConfig); err != nil {
		return trace.Wrap(err)
	}

	if err := sshdConfigTmp.Sync(); err != nil {
		return trace.Wrap(err)
	}

	cmd := exec.Command(sshdBinary, "-t", "-f", sshdConfigTmp.Name())
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "teleport generated an invalid ssh config file, not writing")
	}

	if err := os.Rename(sshdConfigTmp.Name(), sshdConfigPath); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

var restartSSHDFn = restartSSHD

func restartSSHD() error {
	cmd := exec.Command("sshd", "-t")
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "teleport generated an invalid ssh config file")
	}

	cmd = exec.Command("systemctl", "restart", "sshd")
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "teleport failed to restart the sshd service")
	}
	return nil
}

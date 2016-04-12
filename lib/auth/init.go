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

package auth

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// InitConfig is auth server init config
type InitConfig struct {
	// Backend is auth backend to use
	Backend backend.Backend

	// Authority is key generator that we use
	Authority Authority

	// HostUUID is a UUID of this host
	HostUUID string

	// DomainName stores the FQDN of the signing CA (its certificate will have this
	// name embedded). It is usually set to the GUID of the host the Auth service runs on
	DomainName string

	// Authorities is a list of pre-configured authorities to supply on first start
	Authorities []services.CertAuthority

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	// DataDir is the full path to the directory where keys, events and logs are kept
	DataDir string

	// AllowedTokens is a static set of allowed provisioning tokens
	// auth server will recognize on the first start
	AllowedTokens map[string]string

	// ReverseTunnels is a list of reverse tunnels statically supplied
	// in configuration, so auth server will init the tunnels on the first start
	ReverseTunnels []services.ReverseTunnel

	// OIDCConnectors is a list of trusted OpenID Connect identity providers
	// in configuration, so auth server will init the tunnels on the first start
	OIDCConnectors []services.OIDCConnector

	// Trust is a service that manages users and credentials
	Trust services.Trust
	// Lock is a distributed or local lock service
	Lock services.Lock
	// Presence service is a discovery and hearbeat tracker
	Presence services.Presence
	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner
	// Trust is a service that manages users and credentials
	Identity services.Identity
}

// Init instantiates and configures an instance of AuthServer
func Init(cfg InitConfig) (*AuthServer, *Identity, error) {
	if cfg.DataDir == "" {
		return nil, nil, trace.BadParameter("DataDir: data dir can not be empty")
	}

	if cfg.HostUUID == "" {
		return nil, nil, trace.BadParameter("HostUUID: host UUID can not be empty")
	}

	err := os.MkdirAll(cfg.DataDir, os.ModeDir|0777)
	if err != nil {
		log.Errorf(err.Error())
		return nil, nil, err
	}

	lockService := local.NewLockService(cfg.Backend)
	err = lockService.AcquireLock(cfg.DomainName, 60*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer lockService.ReleaseLock(cfg.DomainName)

	// check that user CA and host CA are present and set the certs if needed
	asrv := NewAuthServer(&cfg)

	// we determine if it's the first start by checking if the CA's are set
	var firstStart bool

	firstStart, err = isFirstStart(asrv, cfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// populate authorities on first start
	if firstStart && len(cfg.Authorities) != 0 {
		log.Infof("FIRST START: populating trusted authorities supplied from configuration")
		for _, ca := range cfg.Authorities {
			if err := asrv.Trust.UpsertCertAuthority(ca, backend.Forever); err != nil {
				return nil, nil, trace.Wrap(err)
			}
		}
	}

	// this block will generate user CA authority on first start if it's
	// not currently present, it will also use optional passed user ca keypair
	// that can be supplied in configuration
	if _, err := asrv.GetCertAuthority(services.CertAuthID{DomainName: cfg.DomainName, Type: services.HostCA}, false); err != nil {
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}

		log.Infof("FIRST START: Generating host CA on first start")
		priv, pub, err := asrv.GenerateKeyPair("")
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		hostCA := services.CertAuthority{
			DomainName:   cfg.DomainName,
			Type:         services.HostCA,
			SigningKeys:  [][]byte{priv},
			CheckingKeys: [][]byte{pub},
		}
		if err := asrv.Trust.UpsertCertAuthority(hostCA, backend.Forever); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	// this block will generate user CA authority on first start if it's
	// not currently present, it will also use optional passed user ca keypair
	// that can be supplied in configuration
	if _, err := asrv.GetCertAuthority(services.CertAuthID{DomainName: cfg.DomainName, Type: services.UserCA}, false); err != nil {
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}

		log.Infof("FIRST START: Generating user CA on first start")
		priv, pub, err := asrv.GenerateKeyPair("")
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		userCA := services.CertAuthority{
			DomainName:   cfg.DomainName,
			Type:         services.UserCA,
			SigningKeys:  [][]byte{priv},
			CheckingKeys: [][]byte{pub},
		}
		if err := asrv.Trust.UpsertCertAuthority(userCA, backend.Forever); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	if firstStart {
		if len(cfg.AllowedTokens) != 0 {
			log.Infof("FIRST START: Setting allowed provisioning tokens")
			for token, domainName := range cfg.AllowedTokens {
				log.Infof("FIRST START: upsert provisioning token: domainName: %v", domainName)
				var role string
				token, role, err = services.SplitTokenRole(token)
				if err != nil {
					return nil, nil, trace.Wrap(err)
				}

				if err := asrv.UpsertToken(token, role, 600*time.Second); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
		}

		if len(cfg.ReverseTunnels) != 0 {
			log.Infof("FIRST START: Initializing reverse tunnels")
			for _, tunnel := range cfg.ReverseTunnels {
				if err := asrv.UpsertReverseTunnel(tunnel, 0); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
		}

		if len(cfg.OIDCConnectors) != 0 {
			log.Infof("FIRST START: Initializing oidc connectors")
			for _, connector := range cfg.OIDCConnectors {
				if err := asrv.UpsertOIDCConnector(connector, 0); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
		}
	}
	identity, err := initKeys(asrv, cfg.DataDir, IdentityID{HostUUID: cfg.HostUUID, Role: teleport.RoleAdmin})
	if err != nil {
		return nil, nil, err
	}

	return asrv, identity, nil
}

func isFirstStart(authServer *AuthServer, cfg InitConfig) (bool, error) {
	_, err := authServer.GetCertAuthority(services.CertAuthID{DomainName: cfg.DomainName, Type: services.HostCA}, false)
	if err != nil {
		if !trace.IsNotFound(err) {
			return false, trace.Wrap(err)
		}
		return true, nil
	}
	return false, nil
}

// initKeys initializes this node's host certificate signed by host authority
func initKeys(a *AuthServer, dataDir string, id IdentityID) (*Identity, error) {
	kp, cp := keysPath(dataDir, id)

	keyExists, err := pathExists(kp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certExists, err := pathExists(cp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !keyExists || !certExists {
		privateKey, publicKey, err := a.GenerateKeyPair("")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert, err := a.GenerateHostCert(publicKey, id.HostUUID, a.DomainName, id.Role, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := writeKeys(dataDir, id, privateKey, cert); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	i, err := ReadIdentity(dataDir, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return i, nil
}

// writeKeys saves the key/cert pair for a given domain onto disk. This usually means the
// domain trusts us (signed our public key)
func writeKeys(dataDir string, id IdentityID, key []byte, cert []byte) error {
	kp, cp := keysPath(dataDir, id)
	log.Debugf("write key to %v, cert from %v", kp, cp)

	if err := ioutil.WriteFile(kp, key, 0600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cp, cert, 0600); err != nil {
		return err
	}
	return nil
}

// Identity is a collection of certificates and signers that represent identity
type Identity struct {
	ID              IdentityID
	KeyBytes        []byte
	CertBytes       []byte
	KeySigner       ssh.Signer
	Cert            *ssh.Certificate
	AuthorityDomain string
}

// IdentityID is a combination of role and host UUID
type IdentityID struct {
	Role     teleport.Role
	HostUUID string
}

// Equals returns true if two identities are equal
func (id *IdentityID) Equals(other IdentityID) bool {
	return id.Role == other.Role && id.HostUUID == other.HostUUID
}

// String returns debug friendly representation of this identity
func (id *IdentityID) String() string {
	return fmt.Sprintf("Identity(hostuuid=%v, role=%v)", id.HostUUID, id.Role)
}

// ReadIdentityFromKeyPair reads identity from initialized keypair
func ReadIdentityFromKeyPair(keyBytes, certBytes []byte) (*Identity, error) {
	if len(keyBytes) == 0 {
		return nil, trace.BadParameter("PrivateKey: missing private key")
	}

	if len(certBytes) == 0 {
		return nil, trace.BadParameter("Cert: missing parameter")
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, trace.BadParameter("failed to parse server certificate: %v", err)
	}

	cert, ok := pubKey.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("expected ssh.Certificate, got %v", pubKey)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, trace.BadParameter("failed to parse private key: %v", err)
	}
	// this signer authenticates using certificate signed by the cert authority
	// not only by the public key
	certSigner, err := ssh.NewCertSigner(cert, signer)
	if err != nil {
		return nil, trace.BadParameter("unsupported private key: %v", err)
	}
	if len(cert.ValidPrincipals) != 1 {
		return nil, trace.BadParameter("valid principals: need exactly 1 valid principal: host uuid")
	}

	if len(cert.Permissions.Extensions) == 0 {
		return nil, trace.BadParameter("extensions: misssing needed extensions for host roles")
	}

	roleString := cert.Permissions.Extensions[utils.CertExtensionRole]
	if roleString == "" {
		return nil, trace.BadParameter("misssing cert extension %v", utils.CertExtensionRole)
	}
	role := teleport.Role(roleString)
	if err := role.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	authorityDomain := cert.Permissions.Extensions[utils.CertExtensionAuthority]
	if authorityDomain == "" {
		return nil, trace.BadParameter("misssing cert extension %v", utils.CertExtensionAuthority)
	}

	if cert.ValidPrincipals[0] == "" {
		return nil, trace.BadParameter("valid principal can not be empty")
	}

	return &Identity{
		ID:              IdentityID{HostUUID: cert.ValidPrincipals[0], Role: role},
		AuthorityDomain: authorityDomain,
		KeyBytes:        keyBytes,
		CertBytes:       certBytes,
		KeySigner:       certSigner,
		Cert:            cert,
	}, nil
}

// ReadIdentity reads, parses and returns the given pub/pri key + cert from the
// key storage (dataDir).
func ReadIdentity(dataDir string, id IdentityID) (i *Identity, err error) {
	kp, cp := keysPath(dataDir, id)
	log.Debugf("host identity: [key: %v, cert: %v]", kp, cp)

	keyBytes, err := utils.ReadPath(kp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certBytes, err := utils.ReadPath(cp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ReadIdentityFromKeyPair(keyBytes, certBytes)
}

// WriteIdentity writes identity keypair to disk
func WriteIdentity(dataDir string, identity *Identity) error {
	return trace.Wrap(
		writeKeys(dataDir, identity.ID, identity.KeyBytes, identity.CertBytes))
}

// HaveHostKeys checks either the host keys are in place
func HaveHostKeys(dataDir string, id IdentityID) (bool, error) {
	kp, cp := keysPath(dataDir, id)

	exists, err := pathExists(kp)
	if !exists || err != nil {
		return exists, err
	}

	exists, err = pathExists(cp)
	if !exists || err != nil {
		return exists, err
	}

	return true, nil
}

// keysPath returns two full file paths: to the host.key and host.cert
func keysPath(dataDir string, id IdentityID) (key string, cert string) {
	return filepath.Join(dataDir, fmt.Sprintf("%s.key", strings.ToLower(string(id.Role)))),
		filepath.Join(dataDir, fmt.Sprintf("%s.cert", strings.ToLower(string(id.Role))))
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

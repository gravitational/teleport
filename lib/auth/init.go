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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
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

	// NodeName is the DNS name of the node
	NodeName string

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

	// ReverseTunnels is a list of reverse tunnels statically supplied
	// in configuration, so auth server will init the tunnels on the first start
	ReverseTunnels []services.ReverseTunnel

	// OIDCConnectors is a list of trusted OpenID Connect identity providers
	// in configuration, so auth server will init the tunnels on the first start
	OIDCConnectors []services.OIDCConnector

	// Trust is a service that manages users and credentials
	Trust services.Trust

	// Presence service is a discovery and hearbeat tracker
	Presence services.Presence

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Identity is a service that manages users and credentials
	Identity services.Identity

	// Access is service controlling access to resources
	Access services.Access

	// ClusterAuthPreferenceService is a service to get and set authentication preferences.
	ClusterAuthPreferenceService services.ClusterAuthPreference

	// UniversalSecondFactorService is a service to get and set universal second factor settings.
	UniversalSecondFactorService services.UniversalSecondFactorSettings

	// Roles is a set of roles to create
	Roles []services.Role

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens []services.ProvisionToken

	// AuthPreference defines the authentication type (local, oidc) and second
	// factor (off, otp, u2f) passed in from a configuration file.
	AuthPreference services.AuthPreference

	// U2F defines U2F application ID and any facets passed in from a configuration file.
	U2F services.UniversalSecondFactor

	// DeveloperMode should only be used during development as it does several
	// unsafe things like log sensitive information to console as well as
	// not verify certificates.
	DeveloperMode bool
}

// Init instantiates and configures an instance of AuthServer
func Init(cfg InitConfig, dynamicConfig bool) (*AuthServer, *Identity, error) {
	if cfg.DataDir == "" {
		return nil, nil, trace.BadParameter("DataDir: data dir can not be empty")
	}
	if cfg.HostUUID == "" {
		return nil, nil, trace.BadParameter("HostUUID: host UUID can not be empty")
	}

	err := cfg.Backend.AcquireLock(cfg.DomainName, 30*time.Second)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer cfg.Backend.ReleaseLock(cfg.DomainName)

	// check that user CA and host CA are present and set the certs if needed
	asrv := NewAuthServer(&cfg)

	// we determine if it's the first start by checking if the CA's are set
	firstStart, err := isFirstStart(asrv, cfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// the logic for when to upload resources to the backend is as follows:
	//
	// if dynamicConfig is false:                   upload resources
	// if dynamicConfig is true AND firstStart:     upload resources
	// if dynamicConfig is true AND NOT firstStart: don't upload resources
	uploadResources := true
	if dynamicConfig == true && firstStart == false {
		uploadResources = false
	}

	if uploadResources {
		err = asrv.SetClusterAuthPreference(cfg.AuthPreference)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		log.Infof("[INIT] Set Cluster Authentication Preference: %v", cfg.AuthPreference)

		if cfg.U2F != nil {
			err = asrv.SetUniversalSecondFactor(cfg.U2F)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			log.Infof("[INIT] Set Universal Second Factor Settings: %v", cfg.U2F)
		}

		if cfg.OIDCConnectors != nil && len(cfg.OIDCConnectors) > 0 {
			for _, connector := range cfg.OIDCConnectors {
				if err := asrv.UpsertOIDCConnector(connector); err != nil {
					return nil, nil, trace.Wrap(err)
				}
				log.Infof("[INIT] Created ODIC Connector: %q", connector.GetName())
			}
		}

		for _, role := range cfg.Roles {
			if err := asrv.UpsertRole(role, backend.Forever); err != nil {
				return nil, nil, trace.Wrap(err)
			}
			log.Infof("[INIT] Created Role: %v", role)
		}

		for i := range cfg.Authorities {
			ca := cfg.Authorities[i]
			ca, err = services.GetCertAuthorityMarshaler().GenerateCertAuthority(ca)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}

			if err := asrv.Trust.UpsertCertAuthority(ca); err != nil {
				return nil, nil, trace.Wrap(err)
			}
			log.Infof("[INIT] Created Trusted Certificate Authority: %q, type: %q", ca.GetName(), ca.GetType())
		}

		for _, tunnel := range cfg.ReverseTunnels {
			if err := asrv.UpsertReverseTunnel(tunnel); err != nil {
				return nil, nil, trace.Wrap(err)
			}
			log.Infof("[INIT] Created Reverse Tunnel: %v", tunnel)
		}
	}

	// always create the default namespace
	err = asrv.UpsertNamespace(services.NewNamespace(defaults.Namespace))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	log.Infof("[INIT] Created Namespace: %q", defaults.Namespace)

	// generate a user certificate authority if it doesn't exist
	if _, err := asrv.GetCertAuthority(services.CertAuthID{DomainName: cfg.DomainName, Type: services.UserCA}, false); err != nil {
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}

		log.Infof("[FIRST START]: Generating user certificate authority (CA)")
		priv, pub, err := asrv.GenerateKeyPair("")
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		userCA := &services.CertAuthorityV2{
			Kind:    services.KindCertAuthority,
			Version: services.V2,
			Metadata: services.Metadata{
				Name:      cfg.DomainName,
				Namespace: defaults.Namespace,
			},
			Spec: services.CertAuthoritySpecV2{
				ClusterName:  cfg.DomainName,
				Type:         services.UserCA,
				SigningKeys:  [][]byte{priv},
				CheckingKeys: [][]byte{pub},
			},
		}

		if err := asrv.Trust.UpsertCertAuthority(userCA); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	// generate a host certificate authority if it doesn't exist
	if _, err := asrv.GetCertAuthority(services.CertAuthID{DomainName: cfg.DomainName, Type: services.HostCA}, false); err != nil {
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}

		log.Infof("[FIRST START]: Generating host certificate authority (CA)")
		priv, pub, err := asrv.GenerateKeyPair("")
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		hostCA := &services.CertAuthorityV2{
			Kind:    services.KindCertAuthority,
			Version: services.V2,
			Metadata: services.Metadata{
				Name:      cfg.DomainName,
				Namespace: defaults.Namespace,
			},
			Spec: services.CertAuthoritySpecV2{
				ClusterName:  cfg.DomainName,
				Type:         services.HostCA,
				SigningKeys:  [][]byte{priv},
				CheckingKeys: [][]byte{pub},
			},
		}

		if err := asrv.Trust.UpsertCertAuthority(hostCA); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	if cfg.DeveloperMode {
		log.Warn("[INIT] Starting Teleport in developer mode. This is dangerous! Sensitive information will be logged to console and certificates will not be verified. Proceed with caution!")
	}

	// read host keys from disk or create them if they don't exist
	iid := IdentityID{
		HostUUID: cfg.HostUUID,
		NodeName: cfg.NodeName,
		Role:     teleport.RoleAdmin,
	}
	identity, err := initKeys(asrv, cfg.DataDir, iid)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// migrate any legacy resources to new format
	err = migrateLegacyResources(cfg, asrv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return asrv, identity, nil
}

func migrateLegacyResources(cfg InitConfig, asrv *AuthServer) error {
	err := migrateUsers(asrv)
	if err != nil {
		return trace.Wrap(err)
	}

	err = migrateCertAuthority(asrv)
	if err != nil {
		return trace.Wrap(err)
	}

	err = migrateAuthPreference(cfg, asrv)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func migrateUsers(asrv *AuthServer) error {
	users, err := asrv.GetUsers()
	if err != nil {
		return trace.Wrap(err)
	}

	for i := range users {
		user := users[i]
		raw, ok := (user.GetRawObject()).(services.UserV1)
		if !ok {
			continue
		}
		log.Infof("[MIGRATION] Legacy User: %v", user.GetName())

		// create role for user and upsert to backend
		role := services.RoleForUser(user)
		role.SetLogins(raw.AllowedLogins)
		err = asrv.UpsertRole(role, backend.Forever)
		if err != nil {
			return trace.Wrap(err)
		}

		// upsert new user to backend
		user.AddRole(role.GetName())
		if err := asrv.UpsertUser(user); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func migrateCertAuthority(asrv *AuthServer) error {
	cas, err := asrv.GetCertAuthorities(services.UserCA, true)
	if err != nil {
		return trace.Wrap(err)
	}

	for i := range cas {
		ca := cas[i]
		raw, ok := (ca.GetRawObject()).(services.CertAuthorityV1)
		if !ok {
			continue
		}

		_, err := asrv.GetRole(services.RoleNameForCertAuthority(ca.GetClusterName()))
		if err == nil {
			continue
		}
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		log.Infof("[MIGRATION] Legacy Certificate Authority: %v", ca.GetName())

		// create role for certificate authority and upsert to backend
		newCA, role := services.ConvertV1CertAuthority(&raw)
		err = asrv.UpsertRole(role, backend.Forever)
		if err != nil {
			return trace.Wrap(err)
		}

		// upsert new certificate authority to backend
		if err := asrv.UpsertCertAuthority(newCA); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func migrateAuthPreference(cfg InitConfig, asrv *AuthServer) error {
	// if no cluster auth preferences exist, upload them from file config
	_, err := asrv.GetClusterAuthPreference()
	if err != nil {
		if trace.IsNotFound(err) {
			err = asrv.SetClusterAuthPreference(cfg.AuthPreference)
			if err != nil {
				return trace.Wrap(err)
			}
			log.Infof("[MIGRATION] Set Cluster Authentication Preference: %v", cfg.AuthPreference)
		} else {
			return trace.Wrap(err)
		}
	}

	// if no u2f settings exist, upload from file config
	if cfg.U2F != nil {
		_, err = asrv.GetUniversalSecondFactor()
		if err != nil {
			if trace.IsNotFound(err) {
				err = asrv.SetUniversalSecondFactor(cfg.U2F)
				if err != nil {
					return trace.Wrap(err)
				}
				log.Infof("[MIGRATION] Set Universal Second Factor Settings: %v", cfg.U2F)
			} else {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// isFirstStart returns 'true' if the auth server is starting for the 1st time
// on this server.
func isFirstStart(authServer *AuthServer, cfg InitConfig) (bool, error) {
	// check if the CA exists?
	_, err := authServer.GetCertAuthority(
		services.CertAuthID{
			DomainName: cfg.DomainName,
			Type:       services.HostCA,
		}, false)
	if err != nil {
		if !trace.IsNotFound(err) {
			return false, trace.Wrap(err)
		}
		// CA not found? --> first start!
		return true, nil
	}
	return false, nil
}

// initKeys initializes a nodes host certificate. If the certificate does not exist, a request
// is made to the certificate authority to generate a host certificate and it's written to disk.
// If a certificate exists on disk, it is read in and returned.
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
		packedKeys, err := a.GenerateServerKeys(id.HostUUID, id.NodeName, teleport.Roles{id.Role})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = writeKeys(dataDir, id, packedKeys.Key, packedKeys.Cert)
		if err != nil {
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
	log.Debugf("[INIT] Writing keys to disk: Key: %q, Cert: %q", kp, cp)

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

// IdentityID is a combination of role, host UUID, and node name.
type IdentityID struct {
	Role     teleport.Role
	HostUUID string
	NodeName string
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

	// check principals on certificate
	if len(cert.ValidPrincipals) < 1 {
		return nil, trace.BadParameter("valid principals: at least one valid principal is required")
	}
	for _, validPrincipal := range cert.ValidPrincipals {
		if validPrincipal == "" {
			return nil, trace.BadParameter("valid principal can not be empty: %q", cert.ValidPrincipals)
		}
	}

	// check permissions on certificate
	if len(cert.Permissions.Extensions) == 0 {
		return nil, trace.BadParameter("extensions: misssing needed extensions for host roles")
	}
	roleString := cert.Permissions.Extensions[utils.CertExtensionRole]
	if roleString == "" {
		return nil, trace.BadParameter("misssing cert extension %v", utils.CertExtensionRole)
	}
	roles, err := teleport.ParseRoles(roleString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	foundRoles := len(roles)
	if foundRoles != 1 {
		return nil, trace.Errorf("expected one role per certificate. found %d: '%s'",
			foundRoles, roles.String())
	}
	role := roles[0]
	authorityDomain := cert.Permissions.Extensions[utils.CertExtensionAuthority]
	if authorityDomain == "" {
		return nil, trace.BadParameter("misssing cert extension %v", utils.CertExtensionAuthority)
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
	log.Debugf("[INIT] Reading keys from disk: Key: %q, Cert: %q", kp, cp)

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

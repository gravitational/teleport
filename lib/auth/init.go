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
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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

	// ClusterName stores the FQDN of the signing CA (its certificate will have this
	// name embedded). It is usually set to the GUID of the host the Auth service runs on
	ClusterName services.ClusterName

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

	// ClusterConfiguration is a services that holds cluster wide configuration.
	ClusterConfiguration services.ClusterConfiguration

	// Roles is a set of roles to create
	Roles []services.Role

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	//StaticTokens []services.ProvisionToken
	StaticTokens services.StaticTokens

	// AuthPreference defines the authentication type (local, oidc) and second
	// factor (off, otp, u2f) passed in from a configuration file.
	AuthPreference services.AuthPreference
}

// Init instantiates and configures an instance of AuthServer
func Init(cfg InitConfig) (*AuthServer, *Identity, error) {
	if cfg.DataDir == "" {
		return nil, nil, trace.BadParameter("DataDir: data dir can not be empty")
	}
	if cfg.HostUUID == "" {
		return nil, nil, trace.BadParameter("HostUUID: host UUID can not be empty")
	}

	domainName := cfg.ClusterName.GetClusterName()
	err := cfg.Backend.AcquireLock(domainName, 30*time.Second)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer cfg.Backend.ReleaseLock(domainName)

	// check that user CA and host CA are present and set the certs if needed
	asrv := NewAuthServer(&cfg)

	// INTERNAL: Authorities (plus Roles) and ReverseTunnels don't follow the
	// same pattern as the rest of the configuration (they are not configuration
	// singletons). However, we need to keep them around while Telekube uses them.
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

	// cluster name can only be set once. if it has already been set and we are
	// trying to update it to something else, hard fail.
	err = asrv.SetClusterName(cfg.ClusterName)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, nil, trace.Wrap(err)
	}
	if trace.IsAlreadyExists(err) {
		cn, err := asrv.GetClusterName()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if cn.GetClusterName() != cfg.ClusterName.GetClusterName() {
			return nil, nil, trace.BadParameter("cannot rename cluster %q to %q: clusters cannot be renamed", cn.GetClusterName(), cfg.ClusterName.GetClusterName())
		}
	}
	log.Debugf("[INIT] Cluster Configuration: %v", cfg.ClusterName)

	err = asrv.SetStaticTokens(cfg.StaticTokens)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	log.Infof("[INIT] Updating Cluster Configuration: %v", cfg.StaticTokens)

	err = asrv.SetAuthPreference(cfg.AuthPreference)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	log.Infof("[INIT] Updating Cluster Configuration: %v", cfg.AuthPreference)

	// always create the default namespace
	err = asrv.UpsertNamespace(services.NewNamespace(defaults.Namespace))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	log.Infof("[INIT] Created Namespace: %q", defaults.Namespace)

	// always create a default admin role
	defaultRole := services.NewAdminRole(lib.IsEnterprise())
	err = asrv.CreateRole(defaultRole, backend.Forever)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, nil, trace.Wrap(err)
	}
	if !trace.IsAlreadyExists(err) {
		log.Infof("[INIT] Created default admin role: %q", defaultRole.GetName())
	}

	// generate a user certificate authority if it doesn't exist
	if _, err := asrv.GetCertAuthority(services.CertAuthID{DomainName: cfg.ClusterName.GetClusterName(), Type: services.UserCA}, false); err != nil {
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
				Name:      cfg.ClusterName.GetClusterName(),
				Namespace: defaults.Namespace,
			},
			Spec: services.CertAuthoritySpecV2{
				ClusterName:  cfg.ClusterName.GetClusterName(),
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
	if _, err := asrv.GetCertAuthority(services.CertAuthID{DomainName: cfg.ClusterName.GetClusterName(), Type: services.HostCA}, false); err != nil {
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
				Name:      cfg.ClusterName.GetClusterName(),
				Namespace: defaults.Namespace,
			},
			Spec: services.CertAuthoritySpecV2{
				ClusterName:  cfg.ClusterName.GetClusterName(),
				Type:         services.HostCA,
				SigningKeys:  [][]byte{priv},
				CheckingKeys: [][]byte{pub},
			},
		}

		if err := asrv.Trust.UpsertCertAuthority(hostCA); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	if lib.IsInsecureDevMode() {
		warningMessage := "[INIT] Starting Teleport in insecure mode. This is " +
			"dangerous! Sensitive information will be logged to console and " +
			"certificates will not be verified. Proceed with caution!"
		log.Warn(warningMessage)
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

	err = migrateRoles(asrv)
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
		role.SetLogins(services.Allow, raw.AllowedLogins)
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

func migrateRoles(asrv *AuthServer) error {
	roles, err := asrv.GetRoles()
	if err != nil {
		return trace.Wrap(err)
	}

	// loop over all roles and only migrate RoleV2 -> RoleV3
	for i, _ := range roles {
		role := roles[i]
		_, ok := (role.GetRawObject()).(services.RoleV2)
		if !ok {
			continue
		}

		// with RoleV2 we never had a TTL so upsert them forever
		err = asrv.UpsertRole(role, backend.Forever)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("[MIGRATION] Updated Role: %v", role.GetName())
	}

	return nil
}

// isFirstStart returns 'true' if the auth server is starting for the 1st time
// on this server.
func isFirstStart(authServer *AuthServer, cfg InitConfig) (bool, error) {
	// check if the CA exists?
	_, err := authServer.GetCertAuthority(
		services.CertAuthID{
			DomainName: cfg.ClusterName.GetClusterName(),
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

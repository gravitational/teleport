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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentAuth,
})

// InitConfig is auth server init config
type InitConfig struct {
	// Backend is auth backend to use
	Backend backend.Backend

	// Authority is key generator that we use
	Authority sshca.Authority

	// KeyStoreConfig is the config for the KeyStore which handles private CA
	// keys that may be held in an HSM.
	KeyStoreConfig keystore.Config

	// HostUUID is a UUID of this host
	HostUUID string

	// NodeName is the DNS name of the node
	NodeName string

	// ClusterName stores the FQDN of the signing CA (its certificate will have this
	// name embedded). It is usually set to the GUID of the host the Auth service runs on
	ClusterName types.ClusterName

	// Authorities is a list of pre-configured authorities to supply on first start
	Authorities []types.CertAuthority

	// Resources is a list of previously backed-up resources used to
	// bootstrap backend on first start.
	Resources []types.Resource

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	// DataDir is the full path to the directory where keys, events and logs are kept
	DataDir string

	// ReverseTunnels is a list of reverse tunnels statically supplied
	// in configuration, so auth server will init the tunnels on the first start
	ReverseTunnels []types.ReverseTunnel

	// OIDCConnectors is a list of trusted OpenID Connect identity providers
	// in configuration, so auth server will init the tunnels on the first start
	OIDCConnectors []types.OIDCConnector

	// Trust is a service that manages users and credentials
	Trust services.Trust

	// Presence service is a discovery and heartbeat tracker
	Presence services.Presence

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Identity is a service that manages users and credentials
	Identity services.Identity

	// Access is service controlling access to resources
	Access services.Access

	// DynamicAccessExt is a service that manages dynamic RBAC.
	DynamicAccessExt services.DynamicAccessExt

	// Events is an event service
	Events types.Events

	// ClusterConfiguration is a services that holds cluster wide configuration.
	ClusterConfiguration services.ClusterConfiguration

	// Restrictions is a service to access network restrictions, etc
	Restrictions services.Restrictions

	// Apps is a service that manages application resources.
	Apps services.Apps

	// Databases is a service that manages database resources.
	Databases services.Databases

	// Roles is a set of roles to create
	Roles []types.Role

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens types.StaticTokens

	// AuthPreference defines the authentication type (local, oidc) and second
	// factor (off, otp, u2f) passed in from a configuration file.
	AuthPreference types.AuthPreference

	// AuditLog is used for emitting events to audit log.
	AuditLog events.IAuditLog

	// ClusterAuditConfig holds cluster audit configuration.
	ClusterAuditConfig types.ClusterAuditConfig

	// ClusterNetworkingConfig holds cluster networking configuration.
	ClusterNetworkingConfig types.ClusterNetworkingConfig

	// SessionRecordingConfig holds session recording configuration.
	SessionRecordingConfig types.SessionRecordingConfig

	// SkipPeriodicOperations turns off periodic operations
	// used in tests that don't need periodic operations.
	SkipPeriodicOperations bool

	// CipherSuites is a list of ciphersuites that the auth server supports.
	CipherSuites []uint16

	// CASigningAlg is a signing algorithm used for SSH (certificate and
	// handshake) signatures for both host and user CAs. This option only
	// affects newly-created CAs.
	CASigningAlg *string

	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter

	// Streamer is events sessionstreamer, used to create continuous
	// session related streams
	Streamer events.Streamer

	// WindowsServices is a service that manages Windows desktop resources.
	WindowsDesktops services.WindowsDesktops
}

// Init instantiates and configures an instance of AuthServer
func Init(cfg InitConfig, opts ...ServerOption) (*Server, error) {
	if cfg.DataDir == "" {
		return nil, trace.BadParameter("DataDir: data dir can not be empty")
	}
	if cfg.HostUUID == "" {
		return nil, trace.BadParameter("HostUUID: host UUID can not be empty")
	}

	ctx := context.TODO()

	domainName := cfg.ClusterName.GetClusterName()
	lock, err := backend.AcquireLock(ctx, cfg.Backend, domainName, 30*time.Second)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer lock.Release(ctx, cfg.Backend)

	// check that user CA and host CA are present and set the certs if needed
	asrv, err := NewServer(&cfg, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if resources are supplied, use them to bootstrap backend state
	// on initial startup.
	if len(cfg.Resources) > 0 {
		firstStart, err := isFirstStart(asrv, cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if firstStart {
			log.Infof("Applying %v bootstrap resources (first initialization)", len(cfg.Resources))
			if err := checkResourceConsistency(asrv.keyStore, domainName, cfg.Resources...); err != nil {
				return nil, trace.Wrap(err, "refusing to bootstrap backend")
			}
			if err := local.CreateResources(ctx, cfg.Backend, cfg.Resources...); err != nil {
				return nil, trace.Wrap(err, "backend bootstrap failed")
			}
		} else {
			log.Warnf("Ignoring %v bootstrap resources (previously initialized)", len(cfg.Resources))
		}
	}

	// Set the ciphersuites that this auth server supports.
	asrv.cipherSuites = cfg.CipherSuites

	// INTERNAL: Authorities (plus Roles) and ReverseTunnels don't follow the
	// same pattern as the rest of the configuration (they are not configuration
	// singletons). However, we need to keep them around while Telekube uses them.
	for _, role := range cfg.Roles {
		if err := asrv.UpsertRole(ctx, role); err != nil {
			return nil, trace.Wrap(err)
		}
		log.Infof("Created role: %v.", role)
	}
	for i := range cfg.Authorities {
		ca := cfg.Authorities[i]
		// Don't re-create CA if it already exists, otherwise
		// the existing cluster configuration will be corrupted;
		// this part of code is only used in tests.
		if err := asrv.Trust.CreateCertAuthority(ca); err != nil {
			if !trace.IsAlreadyExists(err) {
				return nil, trace.Wrap(err)
			}
		} else {
			log.Infof("Created trusted certificate authority: %q, type: %q.", ca.GetName(), ca.GetType())
		}
	}
	for _, tunnel := range cfg.ReverseTunnels {
		if err := asrv.UpsertReverseTunnel(tunnel); err != nil {
			return nil, trace.Wrap(err)
		}
		log.Infof("Created reverse tunnel: %v.", tunnel)
	}

	err = asrv.SetClusterAuditConfig(ctx, cfg.ClusterAuditConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = initSetClusterNetworkingConfig(ctx, asrv, cfg.ClusterNetworkingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = initSetSessionRecordingConfig(ctx, asrv, cfg.SessionRecordingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = initSetAuthPreference(ctx, asrv, cfg.AuthPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The first Auth Server that starts gets to set the name of the cluster.
	// If a cluster name/ID is already stored in the backend, the attempt to set
	// a new name returns an AlreadyExists error.
	err = asrv.SetClusterName(cfg.ClusterName)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	// If the cluster name has already been set, log a warning if the user
	// is trying to change the name.
	if trace.IsAlreadyExists(err) {
		// Get current name of cluster from the backend.
		cn, err := asrv.ClusterConfiguration.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if cn.GetClusterName() != cfg.ClusterName.GetClusterName() {
			warnMessage := "Cannot rename cluster to %q: continuing with %q. Teleport " +
				"clusters can not be renamed once they are created. You are seeing this " +
				"warning for one of two reasons. Either you have not set \"cluster_name\" in " +
				"Teleport configuration and changed the hostname of the auth server or you " +
				"are trying to change the value of \"cluster_name\"."
			log.Warnf(warnMessage,
				cfg.ClusterName.GetClusterName(),
				cn.GetClusterName())
		}
		// Override user passed in cluster name with what is in the backend.
		cfg.ClusterName = cn
	}
	log.Debugf("Cluster configuration: %v.", cfg.ClusterName)

	err = asrv.SetStaticTokens(cfg.StaticTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("Updating cluster configuration: %v.", cfg.StaticTokens)

	// always create the default namespace
	err = asrv.UpsertNamespace(types.DefaultNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("Created namespace: %q.", apidefaults.Namespace)

	// Migrate Host CA as Database CA before certificates generation. Otherwise, the Database CA will be
	// generated which we don't want for existing installations.
	if err := migrateDBAuthority(asrv); err != nil {
		return nil, trace.Wrap(err, "failed to migrate database CA")
	}

	// generate certificate authorities if they don't exist
	for _, caType := range types.CertAuthTypes {
		caID := types.CertAuthID{Type: caType, DomainName: cfg.ClusterName.GetClusterName()}
		ca, err := asrv.GetCertAuthority(caID, true)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			log.Infof("First start: generating %s certificate authority.", caID.Type)
			if err := asrv.createSelfSignedCA(caID); err != nil {
				return nil, trace.Wrap(err)
			}
		} else {
			// Already have a CA. Make sure the keyStore has local keys.
			if !asrv.keyStore.HasLocalActiveKeys(ca) {
				// This could be one of a few cases:
				// 1. A new auth server with an HSM being added to an HA cluster.
				// 2. A new auth server with no HSM being added to an HA cluster
				//    where all current auth servers have HSMs.
				// 3. An existing auth server has restarted with a new HSM configured.
				// 4. An existing HSM auth server has restarted no HSM configured.
				// 5. An existing HSM auth server has restarted with a new UUID.
				if ca.GetType() == types.HostCA {
					// We need local keys to sign the Admin identity to support
					// tctl. For this special case we add AdditionalTrustedKeys
					// without any active keys. These keys will not be used for
					// any signing operations until a CA rotation. Only the Host
					// CA is necessary to issue the Admin identity.
					if err := asrv.ensureLocalAdditionalKeys(ca); err != nil {
						return nil, trace.Wrap(err)
					}
					// reload updated CA for below checks
					if ca, err = asrv.GetCertAuthority(caID, true); err != nil {
						return nil, trace.Wrap(err)
					}
				}
			}
			if !asrv.keyStore.HasLocalActiveKeys(ca) && asrv.keyStore.HasLocalAdditionalKeys(ca) {
				log.Warnf("This auth server has a newly added or removed HSM and will not " +
					"be able to perform any signing operations. You must rotate all CAs " +
					"before routing traffic to this auth server. See https://goteleport.com/docs/admin-guide/#certificate-rotation")
			}
			if !ca.AllKeyTypesMatch() {
				log.Warnf("%s CA contains a combination of raw and PKCS#11 keys. If you are attempting to"+
					" configure HSM support, make sure it is configured on all auth servers in this"+
					" cluster and then perform a CA rotation: https://goteleport.com/docs/admin-guide/#certificate-rotation", caID.Type)
			}
		}
	}

	// Delete any unused keys from the keyStore. This is to avoid exhausting
	// (or wasting) HSM resources.
	if err := asrv.deleteUnusedKeys(); err != nil {
		return nil, trace.Wrap(err)
	}

	if lib.IsInsecureDevMode() {
		warningMessage := "Starting teleport in insecure mode. This is " +
			"dangerous! Sensitive information will be logged to console and " +
			"certificates will not be verified. Proceed with caution!"
		log.Warn(warningMessage)
	}

	// Migrate any legacy resources to new format.
	err = migrateLegacyResources(ctx, asrv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create presets - convenience and example resources.
	err = createPresets(asrv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.SkipPeriodicOperations {
		log.Infof("Auth server is running periodic operations.")
		go asrv.runPeriodicOperations()
	} else {
		log.Infof("Auth server is skipping periodic operations.")
	}

	return asrv, nil
}

func initSetAuthPreference(ctx context.Context, asrv *Server, newAuthPref types.AuthPreference) error {
	storedAuthPref, err := asrv.GetAuthPreference(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	shouldReplace, err := shouldInitReplaceResourceWithOrigin(storedAuthPref, newAuthPref)
	if err != nil {
		return trace.Wrap(err)
	}
	if shouldReplace {
		if err := asrv.SetAuthPreference(ctx, newAuthPref); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Updating cluster auth preference: %v.", newAuthPref)
	}
	return nil
}

func initSetClusterNetworkingConfig(ctx context.Context, asrv *Server, newNetConfig types.ClusterNetworkingConfig) error {
	storedNetConfig, err := asrv.GetClusterNetworkingConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	shouldReplace, err := shouldInitReplaceResourceWithOrigin(storedNetConfig, newNetConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	if shouldReplace {
		if err := asrv.SetClusterNetworkingConfig(ctx, newNetConfig); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Updating cluster networking configuration: %v.", newNetConfig)
	}
	return nil
}

func initSetSessionRecordingConfig(ctx context.Context, asrv *Server, newRecConfig types.SessionRecordingConfig) error {
	storedRecConfig, err := asrv.GetSessionRecordingConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	shouldReplace, err := shouldInitReplaceResourceWithOrigin(storedRecConfig, newRecConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	if shouldReplace {
		if err := asrv.SetSessionRecordingConfig(ctx, newRecConfig); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Updating session recording configuration: %v.", newRecConfig)
	}
	return nil
}

// shouldInitReplaceResourceWithOrigin determines whether the candidate
// resource should be used to replace the stored resource during auth server
// initialization.  Dynamically configured resources must not be overwritten
// when the corresponding file config is left unspecified (i.e., by defaults).
func shouldInitReplaceResourceWithOrigin(stored, candidate types.ResourceWithOrigin) (bool, error) {
	if candidate == nil || (candidate.Origin() != types.OriginDefaults && candidate.Origin() != types.OriginConfigFile) {
		return false, trace.BadParameter("candidate origin must be either defaults or config-file (this is a bug)")
	}

	// If there is no resource stored in the backend or it was not dynamically
	// configured, the candidate resource should be stored in the backend.
	if stored == nil || stored.Origin() != types.OriginDynamic {
		return true, nil
	}

	// If the candidate resource is explicitly configured in the config file,
	// store this config-file resource in the backend no matter what.
	if candidate.Origin() == types.OriginConfigFile {
		// Log a warning when about to overwrite a dynamically configured resource.
		if stored.Origin() == types.OriginDynamic {
			log.Warnf("Stored %v resource that was configured dynamically is about to be discarded in favor of explicit file configuration.", stored.GetKind())
		}
		return true, nil
	}

	// The resource in the backend was configured dynamically, and there is no
	// more authoritative file configuration to replace it.  Keep the stored
	// dynamic resource.
	return false, nil
}

func migrateLegacyResources(ctx context.Context, asrv *Server) error {
	if err := migrateRemoteClusters(ctx, asrv); err != nil {
		return trace.Wrap(err)
	}
	if err := migrateCertAuthorities(ctx, asrv); err != nil {
		return trace.Wrap(err, "fail to migrate certificate authorities to the v7 storage format: %v; please report this at https://github.com/gravitational/teleport/issues/new?assignees=&labels=bug&template=bug_report.md including the *redacted* output of 'tctl get cert_authority'", err)
	}
	return nil
}

// createPresets creates preset resources (eg, roles).
func createPresets(asrv *Server) error {
	roles := []types.Role{
		services.NewPresetEditorRole(),
		services.NewPresetAccessRole(),
		services.NewPresetAuditorRole(),
	}
	for _, role := range roles {
		err := asrv.CreateRole(role)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.WrapWithMessage(err, "failed to create preset role %v", role.GetName())
			}
		}
	}
	return nil
}

// isFirstStart returns 'true' if the auth server is starting for the 1st time
// on this server.
func isFirstStart(authServer *Server, cfg InitConfig) (bool, error) {
	// check if the CA exists?
	_, err := authServer.GetCertAuthority(
		types.CertAuthID{
			DomainName: cfg.ClusterName.GetClusterName(),
			Type:       types.HostCA,
		}, false)
	if err != nil {
		if !trace.IsNotFound(err) {
			return false, trace.Wrap(err)
		}
		// If a CA was not found, that means this is the first start.
		return true, nil
	}
	return false, nil
}

// checkResourceConsistency checks far basic conflicting state issues.
func checkResourceConsistency(keyStore keystore.KeyStore, clusterName string, resources ...types.Resource) error {
	for _, rsc := range resources {
		switch r := rsc.(type) {
		case types.CertAuthority:
			// check that signing CAs have expected cluster name and that
			// all CAs for this cluster do having signing keys.
			seemsLocal := r.GetClusterName() == clusterName

			var hasKeys bool
			var signerErr error
			switch r.GetType() {
			case types.HostCA, types.UserCA:
				_, signerErr = keyStore.GetSSHSigner(r)
			case types.DatabaseCA:
				_, _, signerErr = keyStore.GetTLSCertAndSigner(r)
			case types.JWTSigner:
				_, signerErr = keyStore.GetJWTSigner(r)
			default:
				return trace.BadParameter("unexpected cert_authority type %s for cluster %v", r.GetType(), clusterName)
			}
			switch {
			case signerErr == nil:
				hasKeys = true
			case trace.IsNotFound(signerErr):
				hasKeys = false
			default:
				return trace.Wrap(signerErr)
			}

			if seemsLocal && !hasKeys {
				return trace.BadParameter("ca for local cluster %q missing signing keys", clusterName)
			}
			if !seemsLocal && hasKeys {
				return trace.BadParameter("unexpected cluster name %q for signing ca (expected %q)", r.GetClusterName(), clusterName)
			}
		case types.TrustedCluster:
			if r.GetName() == clusterName {
				return trace.BadParameter("trusted cluster has same name as local cluster (%q)", clusterName)
			}
		default:
			// No validation checks for this resource type
		}
	}
	return nil
}

// GenerateIdentity generates identity for the auth server
func GenerateIdentity(a *Server, id IdentityID, additionalPrincipals, dnsNames []string) (*Identity, error) {
	priv, pub, err := a.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPub, err := PrivateKeyToPublicKeyTLS(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := a.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               id.HostUUID,
			NodeName:             id.NodeName,
			Role:                 id.Role,
			AdditionalPrincipals: additionalPrincipals,
			DNSNames:             dnsNames,
			PublicSSHKey:         pub,
			PublicTLSKey:         tlsPub,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ReadIdentityFromKeyPair(priv, certs)
}

// Identity is collection of certificates and signers that represent server identity
type Identity struct {
	// ID specifies server unique ID, name and role
	ID IdentityID
	// KeyBytes is a PEM encoded private key
	KeyBytes []byte
	// CertBytes is a PEM encoded SSH host cert
	CertBytes []byte
	// TLSCertBytes is a PEM encoded TLS x509 client certificate
	TLSCertBytes []byte
	// TLSCACertBytes is a list of PEM encoded TLS x509 certificate of certificate authority
	// associated with auth server services
	TLSCACertsBytes [][]byte
	// SSHCACertBytes is a list of SSH CAs encoded in the authorized_keys format.
	SSHCACertBytes [][]byte
	// KeySigner is an SSH host certificate signer
	KeySigner ssh.Signer
	// Cert is a parsed SSH certificate
	Cert *ssh.Certificate
	// XCert is X509 client certificate
	XCert *x509.Certificate
	// ClusterName is a name of host's cluster
	ClusterName string
}

// String returns user-friendly representation of the identity.
func (i *Identity) String() string {
	var out []string
	if i.XCert != nil {
		out = append(out, fmt.Sprintf("cert(%v issued by %v:%v)", i.XCert.Subject.CommonName, i.XCert.Issuer.CommonName, i.XCert.Issuer.SerialNumber))
	}
	for j := range i.TLSCACertsBytes {
		cert, err := tlsca.ParseCertificatePEM(i.TLSCACertsBytes[j])
		if err != nil {
			out = append(out, err.Error())
		} else {
			out = append(out, fmt.Sprintf("trust root(%v:%v)", cert.Subject.CommonName, cert.Subject.SerialNumber))
		}
	}
	return fmt.Sprintf("Identity(%v, %v)", i.ID.Role, strings.Join(out, ","))
}

// CertInfo returns diagnostic information about certificate
func CertInfo(cert *x509.Certificate) string {
	return fmt.Sprintf("cert(%v issued by %v:%v)", cert.Subject.CommonName, cert.Issuer.CommonName, cert.Issuer.SerialNumber)
}

// TLSCertInfo returns diagnostic information about certificate
func TLSCertInfo(cert *tls.Certificate) string {
	x509cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return err.Error()
	}
	return CertInfo(x509cert)
}

// CertAuthorityInfo returns debugging information about certificate authority
func CertAuthorityInfo(ca types.CertAuthority) string {
	var out []string
	for _, keyPair := range ca.GetTrustedTLSKeyPairs() {
		cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
		if err != nil {
			out = append(out, err.Error())
		} else {
			out = append(out, fmt.Sprintf("trust root(%v:%v)", cert.Subject.CommonName, cert.Subject.SerialNumber))
		}
	}
	return fmt.Sprintf("cert authority(state: %v, phase: %v, roots: %v)", ca.GetRotation().State, ca.GetRotation().Phase, strings.Join(out, ", "))
}

// HasTLSConfig returns true if this identity has TLS certificate and private
// key.
func (i *Identity) HasTLSConfig() bool {
	return len(i.TLSCACertsBytes) != 0 && len(i.TLSCertBytes) != 0
}

// HasPrincipals returns whether identity has principals
func (i *Identity) HasPrincipals(additionalPrincipals []string) bool {
	set := utils.StringsSet(i.Cert.ValidPrincipals)
	for _, principal := range additionalPrincipals {
		if _, ok := set[principal]; !ok {
			return false
		}
	}
	return true
}

// HasDNSNames returns true if TLS certificate has required DNS names
func (i *Identity) HasDNSNames(dnsNames []string) bool {
	if i.XCert == nil {
		return false
	}
	set := utils.StringsSet(i.XCert.DNSNames)
	for _, dnsName := range dnsNames {
		if _, ok := set[dnsName]; !ok {
			return false
		}
	}
	return true
}

// TLSConfig returns TLS config for mutual TLS authentication
// can return NotFound error if there are no TLS credentials setup for identity
func (i *Identity) TLSConfig(cipherSuites []uint16) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	if !i.HasTLSConfig() {
		return nil, trace.NotFound("no TLS credentials setup for this identity")
	}
	tlsCert, err := tls.X509KeyPair(i.TLSCertBytes, i.KeyBytes)
	if err != nil {
		return nil, trace.BadParameter("failed to parse private key: %v", err)
	}
	certPool := x509.NewCertPool()
	for j := range i.TLSCACertsBytes {
		parsedCert, err := tlsca.ParseCertificatePEM(i.TLSCACertsBytes[j])
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse CA certificate")
		}
		certPool.AddCert(parsedCert)
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ClientCAs = certPool
	tlsConfig.ServerName = apiutils.EncodeClusterName(i.ClusterName)
	return tlsConfig, nil
}

func (i *Identity) getSSHCheckers() ([]ssh.PublicKey, error) {
	checkers, err := apisshutils.ParseAuthorizedKeys(i.SSHCACertBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checkers, nil
}

// SSHClientConfig returns a ssh.ClientConfig used by nodes to connect to
// the reverse tunnel server.
func (i *Identity) SSHClientConfig(fips bool) (*ssh.ClientConfig, error) {
	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: i.getSSHCheckers,
			FIPS:            fips,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ssh.ClientConfig{
		User:            i.ID.HostUUID,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(i.KeySigner)},
		HostKeyCallback: callback,
		Timeout:         apidefaults.DefaultDialTimeout,
	}, nil
}

// IdentityID is a combination of role, host UUID, and node name.
type IdentityID struct {
	Role     types.SystemRole
	HostUUID string
	NodeName string
}

// HostID is host ID part of the host UUID that consists cluster name
func (id *IdentityID) HostID() (string, error) {
	parts := strings.Split(id.HostUUID, ".")
	if len(parts) < 2 {
		return "", trace.BadParameter("expected 2 parts in %q", id.HostUUID)
	}
	return parts[0], nil
}

// Equals returns true if two identities are equal
func (id *IdentityID) Equals(other IdentityID) bool {
	return id.Role == other.Role && id.HostUUID == other.HostUUID
}

// String returns debug friendly representation of this identity
func (id *IdentityID) String() string {
	return fmt.Sprintf("Identity(hostuuid=%v, role=%v)", id.HostUUID, id.Role)
}

// ReadIdentityFromKeyPair reads SSH and TLS identity from key pair.
func ReadIdentityFromKeyPair(privateKey []byte, certs *proto.Certs) (*Identity, error) {
	identity, err := ReadSSHIdentityFromKeyPair(privateKey, certs.SSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(certs.SSHCACerts) != 0 {
		identity.SSHCACertBytes = certs.SSHCACerts
	}

	if len(certs.TLSCACerts) != 0 {
		// Parse the key pair to verify that identity parses properly for future use.
		i, err := ReadTLSIdentityFromKeyPair(privateKey, certs.TLS, certs.TLSCACerts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		identity.XCert = i.XCert
		identity.TLSCertBytes = certs.TLS
		identity.TLSCACertsBytes = certs.TLSCACerts
	}

	return identity, nil
}

// ReadTLSIdentityFromKeyPair reads TLS identity from key pair
func ReadTLSIdentityFromKeyPair(keyBytes, certBytes []byte, caCertsBytes [][]byte) (*Identity, error) {
	if len(keyBytes) == 0 {
		return nil, trace.BadParameter("missing private key")
	}

	if len(certBytes) == 0 {
		return nil, trace.BadParameter("missing certificate")
	}

	cert, err := tlsca.ParseCertificatePEM(certBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS certificate")
	}

	id, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(cert.Issuer.Organization) == 0 {
		return nil, trace.BadParameter("missing CA organization")
	}

	clusterName := cert.Issuer.Organization[0]
	if clusterName == "" {
		return nil, trace.BadParameter("missing cluster name")
	}
	identity := &Identity{
		ID:              IdentityID{HostUUID: id.Username, Role: types.SystemRole(id.Groups[0])},
		ClusterName:     clusterName,
		KeyBytes:        keyBytes,
		TLSCertBytes:    certBytes,
		TLSCACertsBytes: caCertsBytes,
		XCert:           cert,
	}
	// The passed in ciphersuites don't appear to matter here since the returned
	// *tls.Config is never actually used?
	_, err = identity.TLSConfig(utils.DefaultCipherSuites())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return identity, nil
}

// ReadSSHIdentityFromKeyPair reads identity from initialized keypair
func ReadSSHIdentityFromKeyPair(keyBytes, certBytes []byte) (*Identity, error) {
	if len(keyBytes) == 0 {
		return nil, trace.BadParameter("PrivateKey: missing private key")
	}

	if len(certBytes) == 0 {
		return nil, trace.BadParameter("Cert: missing parameter")
	}

	cert, err := apisshutils.ParseCertificate(certBytes)
	if err != nil {
		return nil, trace.BadParameter("failed to parse server certificate: %v", err)
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
		return nil, trace.BadParameter("extensions: missing needed extensions for host roles")
	}
	roleString := cert.Permissions.Extensions[utils.CertExtensionRole]
	if roleString == "" {
		return nil, trace.BadParameter("misssing cert extension %v", utils.CertExtensionRole)
	}
	roles, err := types.ParseTeleportRoles(roleString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	foundRoles := len(roles)
	if foundRoles != 1 {
		return nil, trace.Errorf("expected one role per certificate. found %d: '%s'",
			foundRoles, roles.String())
	}
	role := roles[0]
	clusterName := cert.Permissions.Extensions[utils.CertExtensionAuthority]
	if clusterName == "" {
		return nil, trace.BadParameter("missing cert extension %v", utils.CertExtensionAuthority)
	}

	return &Identity{
		ID:          IdentityID{HostUUID: cert.ValidPrincipals[0], Role: role},
		ClusterName: clusterName,
		KeyBytes:    keyBytes,
		CertBytes:   certBytes,
		KeySigner:   certSigner,
		Cert:        cert,
	}, nil
}

// ReadLocalIdentity reads, parses and returns the given pub/pri key + cert from the
// key storage (dataDir).
func ReadLocalIdentity(dataDir string, id IdentityID) (*Identity, error) {
	storage, err := NewProcessStorage(context.TODO(), dataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer storage.Close()
	return storage.ReadIdentity(IdentityCurrent, id.Role)
}

// DELETE IN: 2.7.0
// NOTE: Sadly, our integration tests depend on this logic
// because they create remote cluster resource. Our integration
// tests should be migrated to use trusted clusters instead of manually
// creating tunnels.
// This migration adds remote cluster resource migrating from 2.5.0
// where the presence of remote cluster was identified only by presence
// of host certificate authority with cluster name not equal local cluster name
func migrateRemoteClusters(ctx context.Context, asrv *Server) error {
	clusterName, err := asrv.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	certAuthorities, err := asrv.GetCertAuthorities(types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// loop over all roles and make sure any v3 roles have permit port
	// forward and forward agent allowed
	for _, certAuthority := range certAuthorities {
		if certAuthority.GetName() == clusterName.GetClusterName() {
			log.Debugf("Migrations: skipping local cluster cert authority %q.", certAuthority.GetName())
			continue
		}
		// remote cluster already exists
		_, err = asrv.GetRemoteCluster(certAuthority.GetName())
		if err == nil {
			log.Debugf("Migrations: remote cluster already exists for cert authority %q.", certAuthority.GetName())
			continue
		}
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// the cert authority is associated with trusted cluster
		_, err = asrv.GetTrustedCluster(ctx, certAuthority.GetName())
		if err == nil {
			log.Debugf("Migrations: trusted cluster resource exists for cert authority %q.", certAuthority.GetName())
			continue
		}
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		remoteCluster, err := types.NewRemoteCluster(certAuthority.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		err = asrv.CreateRemoteCluster(remoteCluster)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
		log.Infof("Migrations: added remote cluster resource for cert authority %q.", certAuthority.GetName())
	}

	return nil
}

// DELETE IN: 8.0.0
// migrateCertAuthorities migrates the keypair storage format in cert
// authorities to the new format.
func migrateCertAuthorities(ctx context.Context, asrv *Server) error {
	var errors []error
	for _, caType := range []types.CertAuthType{types.HostCA, types.UserCA, types.JWTSigner} {
		cas, err := asrv.GetCertAuthorities(caType, true)
		if err != nil {
			errors = append(errors, trace.Wrap(err, "fetching %v CAs", caType))
			continue
		}
		for _, ca := range cas {
			if err := migrateCertAuthority(asrv, ca); err != nil {
				errors = append(errors, trace.Wrap(err, "failed to migrate %v: %v", ca, err))
				continue
			}
		}
	}
	if len(errors) > 0 {
		log.Errorf("Failed to migrate certificate authorities to the v7 storage format.")
		log.Errorf("Please report the *exact* errors below and *redacted* output of 'tctl get cert_authority' at https://github.com/gravitational/teleport/issues/new?assignees=&labels=bug&template=bug_report.md")
		for _, err := range errors {
			log.Errorf("    %v", err)
		}
		return trace.Errorf("fail to migrate certificate authorities to the v7 storage format")
	}
	return nil
}

// migrateDBAuthority copies Host CA as Database CA. Before v9.0 database access was using host CA to sign all
// DB certificates. In order to support existing installations Teleport copies Host CA as Database CA on
// the first run after update to v9.0+.
// Function does nothing for databases created with Teleport v9.0+.
// https://github.com/gravitational/teleport/issues/5029
//
// DELETE IN 10.0
func migrateDBAuthority(asrv *Server) error {
	clusterName, err := asrv.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	dbCaID := types.CertAuthID{Type: types.DatabaseCA, DomainName: clusterName.GetClusterName()}
	_, err = asrv.GetCertAuthority(dbCaID, false)
	if err == nil {
		return nil // no migration needed. DB cert already exists.
	}
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// Database CA doesn't exist, check for Host.
	hostCaID := types.CertAuthID{Type: types.HostCA, DomainName: clusterName.GetClusterName()}
	hostCA, err := asrv.GetCertAuthority(hostCaID, true)
	if trace.IsNotFound(err) {
		// DB CA and Host CA are missing. Looks like the first start. No migration needed.
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// Database CA is missing, but Host CA has been found. Database was created with pre v9.
	// Copy the Host CA as Database CA.
	log.Infof("Migrating Database CA")

	cav2, ok := hostCA.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("expected host CA to be of *types.CertAuthorityV2 type, got: %T", hostCA)
	}

	dbCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseCA,
		ClusterName: clusterName.GetClusterName(),
		ActiveKeys: types.CAKeySet{
			// Copy only TLS keys as SSH are not needed.
			TLS: cav2.Spec.ActiveKeys.TLS,
		},
		SigningAlg: cav2.Spec.SigningAlg,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = asrv.Trust.CreateCertAuthority(dbCA)
	switch {
	case trace.IsAlreadyExists(err):
		// Probably another auth server have created the DB CA since we last check.
		// This shouldn't be a problem, but let's log it to know when it happens.
		log.Warn("DB CA has already been created by a different Auth server instance")
	case err != nil:
		return trace.Wrap(err)
	}

	return nil
}

func migrateCertAuthority(asrv *Server, ca types.CertAuthority) error {
	// Check if we need to migrate.
	if needsMigration, err := services.CertAuthorityNeedsMigration(ca); err != nil || !needsMigration {
		return trace.Wrap(err)
	}
	log.Infof("Migrating %v to 7.0 storage format.", ca)
	// CA rotation can cause weird edge cases during migration, don't allow
	// rotation and migration in parallel.
	if ca.GetRotation().State == types.RotationStateInProgress {
		return trace.BadParameter("CA rotation is in progress; please finish CA rotation before upgrading teleport")
	}

	if err := services.SyncCertAuthorityKeys(ca); err != nil {
		return trace.Wrap(err)
	}

	// Sanity-check and upsert the modified CA.
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err, "the migrated CA is invalid: %v", err)
	}
	if err := asrv.UpsertCertAuthority(ca); err != nil {
		return trace.Wrap(err, "failed storing the migrated CA: %v", err)
	}
	log.Infof("Successfully migrated %v to 7.0 storage format.", ca)
	return nil
}

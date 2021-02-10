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

package native

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentKeyGen,
})

// PrecomputedNum is the number of keys to precompute and keep cached.
var PrecomputedNum = 25

type keyPair struct {
	privPem  []byte
	pubBytes []byte
}

// keygen is a key generator that precomputes keys to provide quick access to
// public/private key pairs.
type Keygen struct {
	keysCh chan keyPair

	ctx             context.Context
	cancel          context.CancelFunc
	precomputeCount int

	// clock is used to control time.
	clock clockwork.Clock
}

// KeygenOption is a functional optional argument for key generator
type KeygenOption func(k *Keygen)

// SetClock sets the clock to use for key generation.
func SetClock(clock clockwork.Clock) KeygenOption {
	return func(k *Keygen) {
		k.clock = clock
	}
}

// PrecomputeKeys sets up a number of private keys to pre-compute
// in background, 0 disables the process
func PrecomputeKeys(count int) KeygenOption {
	return func(k *Keygen) {
		k.precomputeCount = count
	}
}

// New returns a new key generator.
func New(ctx context.Context, opts ...KeygenOption) *Keygen {
	ctx, cancel := context.WithCancel(ctx)
	k := &Keygen{
		ctx:             ctx,
		cancel:          cancel,
		precomputeCount: PrecomputedNum,
		clock:           clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(k)
	}

	if k.precomputeCount > 0 {
		log.Debugf("SSH cert authority is going to pre-compute %v keys.", k.precomputeCount)
		k.keysCh = make(chan keyPair, k.precomputeCount)
		go k.precomputeKeys()
	} else {
		log.Debugf("SSH cert authority started with no keys pre-compute.")
	}

	return k
}

// Close stops the precomputation of keys (if enabled) and releases all resources.
func (k *Keygen) Close() {
	k.cancel()
}

// GetNewKeyPairFromPool returns precomputed key pair from the pool.
func (k *Keygen) GetNewKeyPairFromPool() ([]byte, []byte, error) {
	select {
	case key := <-k.keysCh:
		return key.privPem, key.pubBytes, nil
	default:
		return GenerateKeyPair("")
	}
}

// precomputeKeys continues loops forever trying to compute cache key pairs.
func (k *Keygen) precomputeKeys() {
	for {
		privPem, pubBytes, err := GenerateKeyPair("")
		if err != nil {
			log.Errorf("Unable to generate key pair: %v.", err)
			continue
		}
		key := keyPair{
			privPem:  privPem,
			pubBytes: pubBytes,
		}

		select {
		case <-k.ctx.Done():
			log.Infof("Stopping key precomputation routine.")
			return
		case k.keysCh <- key:
			continue
		}
	}
}

// GenerateKeyPair returns fresh priv/pub keypair, takes about 300ms to
// execute.
func GenerateKeyPair(passphrase string) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	if err != nil {
		return nil, nil, err
	}
	privDer := x509.MarshalPKCS1PrivateKey(priv)
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDer,
	}
	privPem := pem.EncodeToMemory(&privBlock)

	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	pubBytes := ssh.MarshalAuthorizedKey(pub)
	return privPem, pubBytes, nil
}

// GenerateKeyPair returns fresh priv/pub keypair, takes about 300ms to
// execute.
func (k *Keygen) GenerateKeyPair(passphrase string) ([]byte, []byte, error) {
	return GenerateKeyPair(passphrase)
}

// GenerateHostCert generates a host certificate with the passed in parameters.
// The private key of the CA to sign the certificate must be provided.
func (k *Keygen) GenerateHostCert(c auth.HostCertParams) ([]byte, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return k.GenerateHostCertWithoutValidation(c)
}

// GenerateHostCertWithoutValidation generates a host certificate with the
// passed in parameters without validating them. For use in tests only.
func (k *Keygen) GenerateHostCertWithoutValidation(c auth.HostCertParams) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicHostKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, err := ssh.ParsePrivateKey(c.PrivateCASigningKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer = sshutils.AlgSigner(signer, c.CASigningAlg)

	// Build a valid list of principals from the HostID and NodeName and then
	// add in any additional principals passed in.
	principals := BuildPrincipals(c.HostID, c.NodeName, c.ClusterName, c.Roles)
	principals = append(principals, c.Principals...)
	if len(principals) == 0 {
		return nil, trace.BadParameter("no principals provided: %v, %v, %v",
			c.HostID, c.NodeName, c.Principals)
	}
	principals = utils.Deduplicate(principals)

	// create certificate
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := k.clock.Now().UTC().Add(c.TTL)
		validBefore = uint64(b.Unix())
	}
	cert := &ssh.Certificate{
		ValidPrincipals: principals,
		Key:             pubKey,
		ValidAfter:      uint64(k.clock.Now().UTC().Add(-1 * time.Minute).Unix()),
		ValidBefore:     validBefore,
		CertType:        ssh.HostCert,
	}
	cert.Permissions.Extensions = make(map[string]string)
	cert.Permissions.Extensions[utils.CertExtensionRole] = c.Roles.String()
	cert.Permissions.Extensions[utils.CertExtensionAuthority] = c.ClusterName

	// sign host certificate with private signing key of certificate authority
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Generated SSH host certificate for role %v with principals: %v.",
		c.Roles, principals)
	return ssh.MarshalAuthorizedKey(cert), nil
}

// GenerateUserCert generates a user certificate with the passed in parameters.
// The private key of the CA to sign the certificate must be provided.
func (k *Keygen) GenerateUserCert(c auth.UserCertParams) ([]byte, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "error validating UserCertParams")
	}
	return k.GenerateUserCertWithoutValidation(c)
}

// GenerateUserCertWithoutValidation generates a user certificate with the
// passed in parameters without validating them. For use in tests only.
func (k *Keygen) GenerateUserCertWithoutValidation(c auth.UserCertParams) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicUserKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := k.clock.Now().UTC().Add(c.TTL)
		validBefore = uint64(b.Unix())
		log.Debugf("generated user key for %v with expiry on (%v) %v", c.AllowedLogins, validBefore, b)
	}
	cert := &ssh.Certificate{
		// we have to use key id to identify teleport user
		KeyId:           c.Username,
		ValidPrincipals: c.AllowedLogins,
		Key:             pubKey,
		ValidAfter:      uint64(k.clock.Now().UTC().Add(-1 * time.Minute).Unix()),
		ValidBefore:     validBefore,
		CertType:        ssh.UserCert,
	}
	cert.Permissions.Extensions = map[string]string{
		teleport.CertExtensionPermitPTY: "",
	}
	if c.PermitX11Forwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitX11Forwarding] = ""
	}
	if c.PermitAgentForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitAgentForwarding] = ""
	}
	if c.PermitPortForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitPortForwarding] = ""
	}
	if c.MFAVerified != "" {
		cert.Permissions.Extensions[teleport.CertExtensionMFAVerified] = c.MFAVerified
	}
	if c.ClientIP != "" {
		cert.Permissions.Extensions[teleport.CertExtensionClientIP] = c.ClientIP
	}
	if c.Impersonator != "" {
		cert.Permissions.Extensions[teleport.CertExtensionImpersonator] = c.Impersonator
	}

	// Add roles, traits, and route to cluster in the certificate extensions if
	// the standard format was requested. Certificate extensions are not included
	// legacy SSH certificates due to a bug in OpenSSH <= OpenSSH 7.1:
	// https://bugzilla.mindrot.org/show_bug.cgi?id=2387
	if c.CertificateFormat == teleport.CertificateFormatStandard {
		traits, err := wrappers.MarshalTraits(&c.Traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(traits) > 0 {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportTraits] = string(traits)
		}
		if len(c.Roles) != 0 {
			roles, err := resource.MarshalCertRoles(c.Roles)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles] = roles
		}
		if c.RouteToCluster != "" {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRouteToCluster] = c.RouteToCluster
		}
		if !c.ActiveRequests.IsEmpty() {
			requests, err := c.ActiveRequests.Marshal()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportActiveRequests] = string(requests)
		}
	}

	signer, err := ssh.ParsePrivateKey(c.PrivateCASigningKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer = sshutils.AlgSigner(signer, c.CASigningAlg)
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.MarshalAuthorizedKey(cert), nil
}

// BuildPrincipals takes a hostID, nodeName, clusterName, and role and builds a list of
// principals to insert into a certificate. This function is backward compatible with
// older clients which means:
//    * If RoleAdmin is in the list of roles, only a single principal is returned: hostID
//    * If nodename is empty, it is not included in the list of principals.
func BuildPrincipals(hostID string, nodeName string, clusterName string, roles teleport.Roles) []string {
	// TODO(russjones): This should probably be clusterName, but we need to
	// verify changing this won't break older clients.
	if roles.Include(teleport.RoleAdmin) {
		return []string{hostID}
	}

	// if no hostID was passed it, the user might be specifying an exact list of principals
	if hostID == "" {
		return []string{}
	}

	// always include the hostID, this is what teleport uses internally to find nodes
	principals := []string{
		fmt.Sprintf("%v.%v", hostID, clusterName),
		hostID,
	}

	// nodeName is the DNS name, this is for OpenSSH interoperability
	if nodeName != "" {
		principals = append(principals, fmt.Sprintf("%s.%s", nodeName, clusterName))
		principals = append(principals, nodeName)
	}

	// Add localhost and loopback addresses to allow connecting to proxy/host
	// on the local machine. This should only matter for quickstart and local
	// development.
	principals = append(principals,
		string(teleport.PrincipalLocalhost),
		string(teleport.PrincipalLoopbackV4),
		string(teleport.PrincipalLoopbackV6),
	)

	// deduplicate (in-case hostID and nodeName are the same) and return
	return utils.Deduplicate(principals)
}

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var (
	// this global configures how many pre-caluclated keypairs to keep in the
	// background (perform key genreation in a separate goroutine, useful for
	// web sesssion for snappy UI)
	PrecalculatedKeysNum = 10

	// only one global copy of 'nauth' exists
	singleton nauth = nauth{
		closeC: make(chan bool),
	}
)

type keyPair struct {
	privPem  []byte
	pubBytes []byte
}

type nauth struct {
	generatedKeysC chan keyPair
	closeC         chan bool
	mutex          sync.Mutex
}

// New returns a pointer to a key generator for production purposes
func New() *nauth {
	singleton.mutex.Lock()
	defer singleton.mutex.Unlock()

	if singleton.generatedKeysC == nil && PrecalculatedKeysNum > 0 {
		singleton.generatedKeysC = make(chan keyPair, PrecalculatedKeysNum)
		go singleton.precalculateKeys()
	}
	return &singleton
}

// Close() closes and re-sets the key generator (better to call it only once,
// when the process is stopping, to avoid costly re-initialization)
func (n *nauth) Close() {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	close(n.closeC)
	n.generatedKeysC = nil
	n.closeC = make(chan bool)
}

// GetNewKeyPairFromPool returns pre-generated keypair from a channel, which
// gets replenished by `precalculateKeys` goroutine
func (n *nauth) GetNewKeyPairFromPool() ([]byte, []byte, error) {
	select {
	case key := <-n.generatedKeysC:
		return key.privPem, key.pubBytes, nil
	default:
		return n.GenerateKeyPair("")
	}
}

func (n *nauth) precalculateKeys() {
	for {
		privPem, pubBytes, err := n.GenerateKeyPair("")
		if err != nil {
			log.Errorf(err.Error())
			continue
		}
		key := keyPair{
			privPem:  privPem,
			pubBytes: pubBytes,
		}

		select {
		case <-n.closeC:
			log.Infof("[KEYS] precalculateKeys() exited")
			return
		case n.generatedKeysC <- key:
			continue
		}
	}
}

// GenerateKeyPair returns fresh priv/pub keypair, takes about 300ms to execute
func (n *nauth) GenerateKeyPair(passphrase string) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
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

func (n *nauth) GenerateHostCert(c services.HostCertParams) ([]byte, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicHostKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, err := ssh.ParsePrivateKey(c.PrivateCASigningKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	principals := buildPrincipals(c.HostID, c.NodeName, c.ClusterName, c.Roles)

	// create certificate
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := time.Now().Add(c.TTL)
		validBefore = uint64(b.Unix())
	}
	cert := &ssh.Certificate{
		ValidPrincipals: principals,
		Key:             pubKey,
		ValidBefore:     validBefore,
		CertType:        ssh.HostCert,
	}
	cert.Permissions.Extensions = make(map[string]string)
	cert.Permissions.Extensions[utils.CertExtensionRole] = c.Roles.String()
	cert.Permissions.Extensions[utils.CertExtensionAuthority] = string(c.ClusterName)

	// sign host certificate with private signing key of certificate authority
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, trace.Wrap(err)
	}

	return ssh.MarshalAuthorizedKey(cert), nil
}

func (n *nauth) GenerateUserCert(c services.UserCertParams) ([]byte, error) {
	if c.TTL < defaults.MinCertDuration {
		return nil, trace.BadParameter("wrong certificate TTL")
	}
	if len(c.AllowedLogins) == 0 {
		return nil, trace.BadParameter("allowedLogins: need allowed OS logins")
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicUserKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := time.Now().Add(c.TTL)
		validBefore = uint64(b.Unix())
		log.Debugf("generated user key for %v with expiry on (%v) %v", c.AllowedLogins, validBefore, b)
	}
	cert := &ssh.Certificate{
		// we have to use key id to identify teleport user
		KeyId:           c.Username,
		ValidPrincipals: c.AllowedLogins,
		Key:             pubKey,
		ValidBefore:     validBefore,
		CertType:        ssh.UserCert,
	}
	cert.Permissions.Extensions = map[string]string{
		teleport.CertExtensionPermitPTY:            "",
		teleport.CertExtensionPermitPortForwarding: "",
	}
	if c.PermitAgentForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitAgentForwarding] = ""
	}
	if len(c.Roles) != 0 {
		// if we are requesting a certificate with support for older versions of OpenSSH
		// don't add roles to certificate extensions, due to a bug in <= OpenSSH 7.1
		// https://bugzilla.mindrot.org/show_bug.cgi?id=2387
		if c.Compatibility != teleport.CompatibilityOldSSH {
			roles, err := services.MarshalCertRoles(c.Roles)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles] = roles
		}
	}
	signer, err := ssh.ParsePrivateKey(c.PrivateCASigningKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.MarshalAuthorizedKey(cert), nil
}

// buildPrincipals takes a hostID, nodeName, clusterName, and role and builds a list of
// principals to insert into a certificate. This function is backward compatible with
// older clients which means:
//    * If RoleAdmin is in the list of roles, only a single principal is returned: hostID
//    * If nodename is empty, it is not included in the list of principals.
func buildPrincipals(hostID string, nodeName string, clusterName string, roles teleport.Roles) []string {
	// TODO(russjones): This should probably be clusterName, but we need to
	// verify changing this won't break older clients.
	if roles.Include(teleport.RoleAdmin) {
		return []string{hostID}
	}

	// always include the hostID, this is what teleport uses internally to find nodes
	principals := []string{
		fmt.Sprintf("%v.%v", hostID, clusterName),
	}

	// nodeName is the DNS name, this is for OpenSSH interoperability
	if nodeName != "" {
		principals = append(principals, fmt.Sprintf("%s.%s", nodeName, clusterName))
		principals = append(principals, nodeName)
	}

	// deduplicate (in-case hostID and nodeName are the same) and return
	return utils.Deduplicate(principals)
}

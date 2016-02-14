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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/session"
	"github.com/gravitational/trace"
	"github.com/mailgun/lemma/secret"
	"golang.org/x/crypto/ssh"
)

type InitConfig struct {
	Backend            *encryptedbk.ReplicatedBackend
	Authority          Authority
	DomainName         string
	AuthDomain         string
	DataDir            string
	SecretKey          string
	AllowedTokens      map[string]string
	TrustedAuthorities []services.CertificateAuthority
	// HostCA is an optional host certificate authority keypair
	HostCA *services.LocalCertificateAuthority
	// UserCA is an optional user certificate authority keypair
	UserCA *services.LocalCertificateAuthority
}

func Init(cfg InitConfig) (*AuthServer, ssh.Signer, error) {

	if cfg.AuthDomain == "" {
		return nil, nil, fmt.Errorf("node name can not be empty")
	}
	if cfg.DataDir == "" {
		return nil, nil, fmt.Errorf("path can not be empty")
	}

	err := os.MkdirAll(cfg.DataDir, os.ModeDir|0777)
	if err != nil {
		log.Errorf(err.Error())
		return nil, nil, err
	}

	lockService := services.NewLockService(cfg.Backend)
	err = lockService.AcquireLock(cfg.AuthDomain, 60*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer lockService.ReleaseLock(cfg.AuthDomain)

	scrt, err := InitSecret(cfg.DataDir, cfg.SecretKey)
	if err != nil {
		return nil, nil, err
	}

	// check that user CA and host CA are present and set the certs if needed
	asrv := NewAuthServer(cfg.Backend, cfg.Authority, scrt, cfg.DomainName)

	// we determine if it's the first start by checking if the CA's are set
	var firstStart bool

	// this block will generate user CA authority on first start if it's
	// not currently present, it will also use optional passed user ca keypair
	// that can be supplied in configuration
	if _, e := asrv.GetHostCertificateAuthority(); e != nil {
		if _, ok := e.(*teleport.NotFoundError); ok {
			firstStart = true
			if cfg.HostCA != nil {
				log.Infof("FIRST START: use host CA keypair provided in config")
				if err := asrv.CAService.UpsertHostCertificateAuthority(*cfg.HostCA); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			} else {
				log.Infof("FIRST START: Generating host CA on first start")
				if err := asrv.ResetHostCertificateAuthority(""); err != nil {
					return nil, nil, err
				}
			}
		} else {
			log.Errorf("Host CA error: %v", e)
			return nil, nil, trace.Wrap(err)
		}
	}

	// this block will generate user CA authority on first start if it's
	// not currently present, it will also use optional passed user ca keypair
	// that can be supplied in configuration
	if _, e := asrv.GetUserCertificateAuthority(); e != nil {
		if _, ok := e.(*teleport.NotFoundError); ok {
			firstStart = true
			if cfg.HostCA != nil {
				log.Infof("FIRST START: use user CA keypair provided in config")
				if err := asrv.CAService.UpsertUserCertificateAuthority(*cfg.UserCA); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			} else {
				log.Infof("FIRST START: Generating user CA on first start")
				if err := asrv.ResetUserCertificateAuthority(""); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}

		} else {
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

				pid, err := session.DecodeSID(session.SecureID(token), scrt)
				if err != nil {
					return nil, nil, trace.Wrap(err)
				}

				if err := asrv.UpsertToken(string(pid), domainName, role, 600*time.Second); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
		}

		if len(cfg.TrustedAuthorities) != 0 {
			log.Infof("FIRST START: Setting trusted certificate authorities")
			for _, cert := range cfg.TrustedAuthorities {
				log.Infof("FIRST START: upsert trusted remote cert: type: %v domainName: %v", cert.Type, cert.DomainName)
				if err := asrv.UpsertRemoteCertificate(cert, 0); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
		}
	}

	signer, err := InitKeys(asrv, cfg.DomainName, cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}

	return asrv, signer, nil
}

// initialize this node's host certificate signed by host authority
func InitKeys(a *AuthServer, domainName, dataDir string) (ssh.Signer, error) {
	if domainName == "" {
		return nil, fmt.Errorf("domainName can not be empty")
	}

	kp, cp := keysPath(domainName, dataDir)

	keyExists, err := pathExists(kp)
	if err != nil {
		return nil, err
	}

	certExists, err := pathExists(cp)
	if err != nil {
		return nil, err
	}

	if !keyExists || !certExists {
		k, pub, err := a.GenerateKeyPair("")
		if err != nil {
			return nil, err
		}
		c, err := a.GenerateHostCert(pub, domainName, domainName, RoleAdmin, 0)
		if err != nil {
			return nil, err
		}
		if err := writeKeys(domainName, dataDir, k, c); err != nil {
			return nil, err
		}
	}
	i, err := ReadIdentity(domainName, dataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return i.KeySigner, nil
}

func writeKeys(domainName, dataDir string, key []byte, cert []byte) error {
	kp, cp := keysPath(domainName, dataDir)

	log.Debugf("write key to %v, cert from %v", kp, cp)

	if err := ioutil.WriteFile(kp, key, 0600); err != nil {
		return err
	}

	if err := ioutil.WriteFile(cp, cert, 0600); err != nil {
		return err
	}

	return nil
}

type Identity struct {
	KeyBytes  []byte
	CertBytes []byte
	KeySigner ssh.Signer
	PubKey    ssh.PublicKey
	Cert      *ssh.Certificate
}

// ReadIdentity reads, parses and returns the given pub/pri key + cert from the
// key storage (dataDir).
func ReadIdentity(hostname, dataDir string) (i *Identity, err error) {
	kp, cp := keysPath(hostname, dataDir)
	log.Debugf("Identity %s: [key: %v, cert: %v]", hostname, kp, cp)

	i = new(Identity)

	i.KeyBytes, err = utils.ReadPath(kp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.CertBytes, err = utils.ReadPath(cp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.PubKey, _, _, _, err = ssh.ParseAuthorizedKey(i.CertBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server CA certificate '%v', err: %v",
			string(i.CertBytes), err)
	}

	var ok bool
	i.Cert, ok = i.PubKey.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected CA certificate, got %T ", i.PubKey)
	}

	signer, err := ssh.ParsePrivateKey(i.KeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host private key, err: %v", err)
	}
	// TODO: why NewCertSigner if we already have a signer from ParsePrivateKey?
	i.KeySigner, err = ssh.NewCertSigner(i.Cert, signer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host private key, err: %v", err)
	}
	return i, nil
}

func HaveKeys(domainName, dataDir string) (bool, error) {
	kp, cp := keysPath(domainName, dataDir)

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

func InitSecret(dataDir, secretKey string) (secret.SecretService, error) {
	keyPath := secretKeyPath(dataDir)
	exists, err := pathExists(keyPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		log.Infof("Secret not found. Writing to %v", keyPath)
		if secretKey == "" {
			log.Infof("Secret key is not supplied, generating")
			k, err := secret.NewKey()
			if err != nil {
				return nil, err
			}
			secretKey = secret.KeyToEncodedString(k)
		} else {
			log.Infof("Using secret key provided with configuration")
		}

		err = ioutil.WriteFile(
			keyPath, []byte(secretKey), 0600)
		if err != nil {
			return nil, err
		}
	}
	log.Infof("Reading secret from %v", keyPath)
	return ReadSecret(dataDir)
}

func ReadSecret(dataDir string) (secret.SecretService, error) {
	keyPath := secretKeyPath(dataDir)
	bytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	key, err := secret.EncodedStringToKey(string(bytes))
	if err != nil {
		return nil, err
	}
	return secret.New(&secret.Config{KeyBytes: key})
}

func secretKeyPath(dataDir string) string {
	return filepath.Join(dataDir, "teleport.secret")
}

func keysPath(domainName, dataDir string) (key string, cert string) {
	key = filepath.Join(dataDir, fmt.Sprintf("%v.key", domainName))
	cert = filepath.Join(dataDir, fmt.Sprintf("%v.cert", domainName))
	return
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

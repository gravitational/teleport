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
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type InitConfig struct {
	Backend            *encryptedbk.ReplicatedBackend
	Authority          Authority
	FQDN               string
	AuthDomain         string
	DataDir            string
	SecretKey          string
	AllowedTokens      map[string]string
	TrustedAuthorities []services.RemoteCert
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
	asrv := NewAuthServer(cfg.Backend, cfg.Authority, scrt)

	// we determine if it's the first start by checking if the CA's are set
	var firstStart bool

	if _, e := asrv.GetHostCAPub(); e != nil {

		if _, ok := e.(*teleport.NotFoundError); ok {
			log.Infof("FIRST START: Generating host CA on first start")
			firstStart = true
			if err := asrv.ResetHostCA(""); err != nil {
				return nil, nil, err
			}
		} else {
			log.Errorf("Host CA error: %v", e)
			return nil, nil, trace.Wrap(err)
		}
	}

	if _, e := asrv.GetUserCAPub(); e != nil {

		if _, ok := e.(*teleport.NotFoundError); ok {
			log.Infof("FIRST START: Generating user CA")
			firstStart = true
			if err := asrv.ResetUserCA(""); err != nil {
				return nil, nil, trace.Wrap(err)
			}
		} else {
			return nil, nil, trace.Wrap(err)
		}
	}

	if firstStart {
		if len(cfg.AllowedTokens) != 0 {
			log.Infof("FIRST START: Setting allowed provisioning tokens")
			for fqdn, token := range cfg.AllowedTokens {
				log.Infof("FIRST START: upsert provisioning token: fqdn: %v", fqdn)
				pid, err := session.DecodeSID(session.SecureID(token), scrt)
				if err != nil {
					return nil, nil, trace.Wrap(err)
				}
				if err := asrv.UpsertToken(string(pid), fqdn, 600*time.Second); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
		}

		if len(cfg.TrustedAuthorities) != 0 {
			log.Infof("FIRST START: Setting trusted certificate authorities")
			for _, cert := range cfg.TrustedAuthorities {
				log.Infof("FIRST START: upsert user cert: type: %v fqdn: %v", cert.Type, cert.FQDN)
				if err := asrv.UpsertRemoteCert(cert, 0); err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
		}
	}

	signer, err := InitKeys(asrv, cfg.FQDN, cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}

	return asrv, signer, nil
}

// initialize this node's host certificate signed by host authority
func InitKeys(a *AuthServer, fqdn, dataDir string) (ssh.Signer, error) {
	if fqdn == "" {
		return nil, fmt.Errorf("fqdn can not be empty")
	}

	kp, cp := keysPath(fqdn, dataDir)

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
		c, err := a.GenerateHostCert(pub, fqdn, fqdn, 0)
		if err != nil {
			return nil, err
		}
		if err := writeKeys(fqdn, dataDir, k, c); err != nil {
			return nil, err
		}
	}
	return ReadKeys(fqdn, dataDir)
}

func writeKeys(fqdn, dataDir string, key []byte, cert []byte) error {
	kp, cp := keysPath(fqdn, dataDir)

	log.Infof("write key to %v, cert from %v", kp, cp)

	if err := ioutil.WriteFile(kp, key, 0600); err != nil {
		return err
	}

	if err := ioutil.WriteFile(cp, cert, 0600); err != nil {
		return err
	}

	return nil
}

func ReadKeys(fqdn, dataDir string) (ssh.Signer, error) {
	kp, cp := keysPath(fqdn, dataDir)

	log.Infof("read key from %v, cert from %v", kp, cp)

	key, err := utils.ReadPath(kp)
	if err != nil {
		return nil, err
	}

	cert, err := utils.ReadPath(cp)
	if err != nil {
		return nil, err
	}

	pk, _, _, _, _ := ssh.ParseAuthorizedKey(cert)
	fmt.Printf("auth key: ", string(ssh.MarshalAuthorizedKey(pk.(*ssh.Certificate).SignatureKey)))

	return sshutils.NewSigner(key, cert)
}

func HaveKeys(fqdn, dataDir string) (bool, error) {
	kp, cp := keysPath(fqdn, dataDir)

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

func keysPath(fqdn, dataDir string) (key string, cert string) {
	key = filepath.Join(dataDir, fmt.Sprintf("%v.key", fqdn))
	cert = filepath.Join(dataDir, fmt.Sprintf("%v.cert", fqdn))
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

package auth

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/services"
	"github.com/gravitational/teleport/sshutils"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func Init(b backend.Backend, a Authority,
	fqdn, authDomain, dataDir string) (*AuthServer, ssh.Signer, error) {

	if authDomain == "" {
		return nil, nil, fmt.Errorf("node name can not be empty")
	}
	if dataDir == "" {
		return nil, nil, fmt.Errorf("path can not be empty")
	}

	lockService := services.NewLockService(b)
	err := lockService.AcquireLock(authDomain, 60*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer lockService.ReleaseLock(authDomain)

	scrt, err := InitSecret(dataDir)
	if err != nil {
		return nil, nil, err
	}

	// check that user CA and host CA are present and set the certs if needed
	asrv := NewAuthServer(b, a, scrt)

	if _, e := asrv.GetHostCAPub(); e != nil {
		log.Infof("Host CA error: %v", e)
		if _, ok := e.(*teleport.NotFoundError); ok {
			log.Infof("Reseting host CA")
			if err := asrv.ResetHostCA(""); err != nil {
				return nil, nil, err
			}
		}
	}

	if _, e := asrv.GetUserCAPub(); e != nil {
		log.Infof("User CA error: %v", e)
		if _, ok := e.(*teleport.NotFoundError); ok {
			log.Infof("Reseting host CA")
			if err := asrv.ResetUserCA(""); err != nil {
				return nil, nil, err
			}
		}
	}

	signer, err := InitKeys(asrv, fqdn, dataDir)
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

func InitSecret(dataDir string) (*secret.Service, error) {
	keyPath := secretKeyPath(dataDir)
	exists, err := pathExists(keyPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		log.Infof("Secret not found. Writing to %v", keyPath)
		k, err := secret.NewKey()
		if err != nil {
			return nil, err
		}
		err = ioutil.WriteFile(
			keyPath, []byte(secret.KeyToEncodedString(k)), 0600)
		if err != nil {
			return nil, err
		}
	}
	log.Infof("Reading secret from %v", keyPath)
	return ReadSecret(dataDir)
}

func ReadSecret(dataDir string) (*secret.Service, error) {
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

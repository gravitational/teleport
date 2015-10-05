// openssh auth is a wrapper around openssh ssh-keygen program implementing Authority interface
package openssh

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
)

func New() *oauth {
	return &oauth{}
}

// oAuth is an authority implementation based on open ssh toolkit
type oauth struct {
}

func (a *oauth) GenerateKeyPair(passphrase string) ([]byte, []byte, error) {
	dir, err := ioutil.TempDir("", "teleport-auth")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)

	cmd := exec.Command("ssh-keygen", "-f", path.Join(dir, "key"), "-N", passphrase)
	if out, e := cmd.CombinedOutput(); e != nil {
		return nil, nil, fmt.Errorf("%v, out: %v", err, string(out))
	}

	priv, err := ioutil.ReadFile(path.Join(dir, "key"))
	if err != nil {
		return nil, nil, err
	}

	pub, err := ioutil.ReadFile(path.Join(dir, "key.pub"))
	if err != nil {
		return nil, nil, err
	}
	return priv, pub, nil
}

// GenerateHostCert implements call to the following command:
// ssh-keygen -s server_ca -I host_auth_server -h -n auth.example.com -V +52w /etc/ssh/ssh_host_rsa_key.pub
func (a *oauth) GenerateHostCert(cakey, key []byte, id, hostname string, ttl time.Duration) ([]byte, error) {
	dir, err := writeKeys(cakey, key)
	defer os.RemoveAll(dir)
	if err != nil {
		return nil, err
	}

	args := []string{"-s", path.Join(dir, "ca_key"), "-I", id, "-h", "-n", hostname}
	args = addTTL(args, ttl)
	args = append(args, path.Join(dir, "key"))

	cmd := exec.Command("ssh-keygen", args...)
	if out, e := cmd.CombinedOutput(); e != nil {
		return nil, fmt.Errorf("%v, out: %v", err, string(out))
	}

	cert, err := ioutil.ReadFile(path.Join(dir, "key-cert.pub"))
	if err != nil {
		return nil, err
	}
	return cert, nil
}

// GenerateUserCert implements call to the following command:
// ssh-keygen -s users_ca -I user_username -n username -V +52w id_rsa.pub
func (a *oauth) GenerateUserCert(pkey, key []byte, id, username string, ttl time.Duration) ([]byte, error) {
	dir, err := writeKeys(pkey, key)
	defer os.RemoveAll(dir)
	if err != nil {
		return nil, err
	}

	args := []string{"-s", path.Join(dir, "ca_key"), "-I", id, "-n", username}
	args = addTTL(args, ttl)
	args = append(args, path.Join(dir, "key"))

	log.Infof("generate certificate: %v", args)

	cmd := exec.Command("ssh-keygen", args...)
	if out, e := cmd.CombinedOutput(); e != nil {
		return nil, fmt.Errorf("%v, out: %v", err, string(out))
	}

	cert, err := ioutil.ReadFile(path.Join(dir, "key-cert.pub"))
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func writeKeys(cakey, key []byte) (string, error) {
	dir, err := ioutil.TempDir("", "teleport-oauth")
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(path.Join(dir, "ca_key"), cakey, 0400); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(path.Join(dir, "key"), key, 0400); err != nil {
		return "", err
	}
	return dir, nil
}

func addTTL(args []string, ttl time.Duration) []string {
	if ttl == 0 {
		return args
	}
	return append(args, "-V", fmt.Sprintf("+%v", time.Now().Format("20060102150405")))
}

package git

import (
	"os"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func configureKnownHosts(hostname string, keys []ssh.PublicKey) (string, error) {
	knownHostsFile, err := os.CreateTemp("", "*")
	if err != nil {
		return "", trace.Wrap(err, "failed creating known hosts file")
	}
	defer knownHostsFile.Close()

	log.Infof("Writing known_hosts file to %s", knownHostsFile.Name())

	addrs := []string{hostname}
	for _, k := range keys {
		log.Infof("processing key %s...", k.Type())
		_, err := knownHostsFile.WriteString(knownhosts.Line(addrs, k) + "\n")
		if err != nil {
			knownHostsFile.Close()
			os.Remove(knownHostsFile.Name())
			return "", trace.Wrap(err, "failed writing known hosts")
		}
	}

	return knownHostsFile.Name(), nil
}

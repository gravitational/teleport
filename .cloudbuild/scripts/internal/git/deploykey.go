package git

import (
	"io/ioutil"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func writeKey(deployKey []byte) (string, error) {
	keyFile, err := ioutil.TempFile("", "*")
	if err != nil {
		return "", trace.Wrap(err, "failed creating keyfile")
	}
	defer keyFile.Close()

	// We don't want *anyone else* reading this key
	err = keyFile.Chmod(0600)
	if err != nil {
		return "", trace.Wrap(err, "failed securing deploy key")
	}

	log.Infof("Writing deploy key to %s", keyFile.Name())

	_, err = keyFile.Write(deployKey)
	if err != nil {
		return "", trace.Wrap(err, "failed writing deploy key")
	}

	// ensure there is a trailing newline in the key, as older versions of the
	// `ssh` client will barf on a key that doesn't have one, but will happily
	// allow multiples
	_, err = keyFile.WriteString("\n")
	if err != nil {
		return "", trace.Wrap(err, "failed formatting key")
	}

	return keyFile.Name(), nil
}

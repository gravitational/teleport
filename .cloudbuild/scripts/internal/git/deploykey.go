package git

import (
	"os"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func writeKey(deployKey []byte) (string, error) {
	// Note that tempfiles are automatically created with 0600, so no-one else
	// should be able to read this.
	keyFile, err := os.CreateTemp("", "*")
	if err != nil {
		return "", trace.Wrap(err, "failed creating keyfile")
	}
	defer keyFile.Close()

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

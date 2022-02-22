package git

import (
	"fmt"
	"io/ioutil"

	"github.com/gravitational/trace"
)

func touchTempFile() (string, error) {
	tempFile, err := ioutil.TempFile("", "*")
	if err != nil {
		return "", trace.Wrap(err, "failed creating known hosts file")
	}
	defer tempFile.Close()

	err = tempFile.Chmod(0600)
	if err != nil {
		return "", trace.Wrap(err, "failed securing known hosts file")
	}

	return tempFile.Name(), nil
}

func configureKnownHosts() (string, error) {
	knownHostsFile, err := touchTempFile()
	if err != nil {
		return "", trace.Wrap(err, "failed creating known_hosts file")
	}

	script := fmt.Sprintf("ssh-keyscan -H github.com > %q 2>/dev/null", knownHostsFile)
	err = run(".", nil, "/bin/bash", "-c", script)
	if err != nil {
		return "", trace.Wrap(err, "failed adding github.com to known hosts")
	}

	// Ideally we would now validate the contents of `knownHostsFile` against the
	// keys published here:
	//	 https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints

	return knownHostsFile, nil
}

// Command with-secrets emulates the behaviour of sops --exec-env, in that it
// takes the decrypted YAML output from `sops -d $drone-secrets-file` and runs
// the supplied program with the secrets injected its environment.
//
// For some reason (either the way we've structured the yaml in the drone
// secrets files, the use use of multi-line secrets, or both), `sops --exec-env`
// doesn't work with our Drone secrets files, so this shim exists to emulate that
// behaviour
//
// For example:
//
//	$ sops -d encrypted-secrets.yaml | with-secrets ./do-the-thing arg1 arg2
//
// ... so that the decrypted secrets are never written to disk.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"gopkg.in/yaml.v2"
)

type secret struct {
	Value string `yaml:"value"`
}

type Secrets struct {
	Secrets map[string]secret `yaml:"secrets"`
}

func main() {
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err.Error())
	}

	secrets := Secrets{}
	err = yaml.Unmarshal(text, &secrets)
	if err != nil {
		log.Fatal(err.Error())
	}

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	for key, value := range secrets.Secrets {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value.Value))
	}

	err = cmd.Run()
	if err != nil {
		log.Fatalf("Run failed: %s", err)
	}
}

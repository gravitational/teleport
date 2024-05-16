/*
Copyright 2022 Gravitational, Inc.

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

// Emulates the behaviour of sops --exec-env, in that it takes the decrypted
// yaml output from `sops -d $drone-secrets-file` and runs the supplied program
// with the secrets injected its environment.
//
// For some reason (either the way we've structured the yaml in the drone
// secrets files, the use use of multi-line secrets, or both), `sops --exec-env`
// doesn't work with our Drone secrets files, so this shim exists to emulate that
// behaviour
//
// For example:
//  $ sops -d encrypted-secrets.yaml | with-secrets ./do-the-thing arg1 arg2
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

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
package command

import (
	"fmt"
	"io/ioutil"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
)

func (cmd *Command) NewKey(filename string) error {
	key, err := secret.NewKey()
	if err != nil {
		cmd.printError(fmt.Errorf("failed to generate key: %v", err))
		return err
	}
	if len(filename) == 0 {
		fmt.Fprintf(cmd.out, "%v\n", secret.KeyToEncodedString(key))
	} else {
		err := ioutil.WriteFile(filename, ([]byte)(secret.KeyToEncodedString(key)), 0700)
		if err != nil {
			cmd.printError(fmt.Errorf("failed to save key: %v", err))
			return err
		}
	}
	return nil
}

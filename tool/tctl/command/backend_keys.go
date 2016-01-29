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
	"path"
	"text/tabwriter"

	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
)

func (cmd *Command) GetBackendKeys() {
	keys, err := cmd.client.GetSealKeys()
	if err != nil {
		cmd.printError(err)
		return
	}
	w := tabwriter.NewWriter(cmd.out, 10, 20, 0, '\t', 0)
	for _, key := range keys {
		s := key.ID + "\t"
		if len(key.PrivateValue) != 0 {
			s += "private\t"
		} else {
			s += "\t"
		}
		s += key.Name + "\t"
		fmt.Fprintln(w, s)
	}
	fmt.Fprintln(w)
	w.Flush()
}

func (cmd *Command) AddNewBackendKey(id string) {
	key, err := cmd.client.GenerateSealKey(id)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Key " + key.ID + " was generated")
}

func (cmd *Command) GenerateBackendKey(name, filename string) {
	key, err := encryptor.GenerateGPGKey(name)
	if err != nil {
		cmd.printError(err)
		return
	}
	pubKey := key.Public()

	b64key, err := encryptedbk.KeyToString(key)
	if err != nil {
		cmd.printError(err)
		return
	}

	b64keyPub, err := encryptedbk.KeyToString(pubKey)
	if err != nil {
		cmd.printError(err)
		return
	}

	if len(filename) == 0 {
		fmt.Fprintf(cmd.out, "\nFull key:\n\n%v\n\nPublic key:\n\n%v\n\n", b64key, b64keyPub)
	} else {
		err := ioutil.WriteFile(filename, []byte(b64key), 0777)
		if err != nil {
			cmd.printError(err)
			return
		}
		err = ioutil.WriteFile(filename+"_pub", []byte(b64keyPub), 0777)
		if err != nil {
			cmd.printError(err)
			return
		}
	}
}

func (cmd *Command) ImportBackendKey(filename string) {
	key, err := encryptedbk.LoadKeyFromFile(filename)
	if err != nil {
		cmd.printError(err)
		return
	}

	err = cmd.client.AddSealKey(key)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Key " + key.ID + " was imported from " + filename)
}

func (cmd *Command) ExportBackendKey(dir, id string) {
	key, err := cmd.client.GetSealKey(id)
	if err != nil {
		cmd.printError(err)
		return
	}
	filename := path.Join(dir, key.ID+".bkey")

	err = encryptedbk.SaveKeyToFile(key, filename)
	if err != nil {
		cmd.printError(fmt.Errorf("failed to save key: %v", err))
		return
	}
	cmd.printOK("Key " + id + " was saved to " + filename)
}

func (cmd *Command) DeleteBackendKey(id string) {
	err := cmd.client.DeleteSealKey(id)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Key " + id + " was deleted")
}

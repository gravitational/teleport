package command

import (
	"fmt"
	"io/ioutil"
	"path"
)

func (cmd *Command) GetBackendKeys() {
	ids, err := cmd.client.GetBackendKeys()
	if err != nil {
		cmd.printError(err)
		return
	}
	res := "This server has these keys:"
	for _, id := range ids {
		res += " " + id
	}
	res += "\n"
	fmt.Fprintf(cmd.out, res)
}

func (cmd *Command) GetRemoteBackendKeys() {
	ids, err := cmd.client.GetRemoteBackendKeys()
	if err != nil {
		cmd.printError(err)
		return
	}
	res := "Remote backend use these keys for encryption:"
	for _, id := range ids {
		res += " " + id
	}
	res += "\n"
	fmt.Fprintf(cmd.out, res)
}

func (cmd *Command) GenerateBackendKey(id string) {
	err := cmd.client.GenerateBackendKey(id)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Key " + id + " was generated")
}

func (cmd *Command) ImportBackendKey(filename string) {
	keyJSON, err := ioutil.ReadFile(filename)
	id, err := cmd.client.AddBackendKey(string(keyJSON))
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Key " + id + " was imported from " + filename)
}

func (cmd *Command) ExportBackendKey(dir, id string) {
	keyJSON, err := cmd.client.GetBackendKey(id)
	if err != nil {
		cmd.printError(err)
		return
	}
	filename := path.Join(dir, id+".bkey")

	err = ioutil.WriteFile(filename, ([]byte)(keyJSON), 0700)
	if err != nil {
		cmd.printError(fmt.Errorf("failed to save key: %v", err))
		return
	}
	cmd.printOK("Key " + id + " was saved to " + filename)
}

func (cmd *Command) DeleteBackendKey(id string) {
	err := cmd.client.DeleteBackendKey(id)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Key " + id + " was deleted")
}

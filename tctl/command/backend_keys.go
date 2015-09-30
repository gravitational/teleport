package command

import (
	"fmt"
	"github.com/gravitational/teleport/backend/encryptedbk"
	"path"
	"text/tabwriter"
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

func (cmd *Command) GenerateBackendKey(id string) {
	key, err := cmd.client.GenerateSealKey(id)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Key " + key.ID + " was generated")
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

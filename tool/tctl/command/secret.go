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

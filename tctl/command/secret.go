package command

import (
	"fmt"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
)

func (cmd *Command) NewKey() {
	key, err := secret.NewKey()
	if err != nil {
		cmd.printError(fmt.Errorf("failed to generate key: %v", err))
		return
	}
	fmt.Fprintf(cmd.out, "%v\n", secret.KeyToEncodedString(key))
}

package command

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/backend"
)

/*func newHostCACommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "hostca",
		Usage: "Operations with host certificate authority",
		Subcommands: []cli.Command{
			{
				Name:  "reset",
				Usage: "Reset host certificate authority keys",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "confirm", Usage: "Automatically apply the operation without confirmation"},
				},
				Action: c.resetHostCA,
			},
			{
				Name:   "pubkey",
				Usage:  "print host certificate authority public key",
				Action: c.getHostCAPub,
			},
		},
	}
}

func newUserCACommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "userca",
		Usage: "Operations with user certificate authority",
		Subcommands: []cli.Command{
			{
				Name:  "reset",
				Usage: "Reset user certificate authority keys",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "confirm", Usage: "Automatically apply the operation without confirmation"},
				},
				Action: c.resetUserCA,
			},
			{
				Name:   "pubkey",
				Usage:  "print user certificate authority public key",
				Action: c.getUserCAPub,
			},
		},
	}
}

func newRemoteCACommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "remoteca",
		Usage: "Operations with remote certificate authority",
		Subcommands: []cli.Command{
			{
				Name:  "upsert",
				Usage: "Upsert remote certificate to trust",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "Certificate id"},
					cli.StringFlag{Name: "fqdn", Usage: "FQDN of the remote party"},
					cli.StringFlag{Name: "type", Usage: "Cert type (host or user)"},
					cli.StringFlag{Name: "path", Usage: "Cert path (reads from stdout if omitted)"},
					cli.DurationFlag{Name: "ttl", Usage: "ttl for certificate to be trusted"},
				},
				Action: c.upsertRemoteCert,
			},
			{
				Name:  "ls",
				Usage: "List trusted remote certificates",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "fqdn", Usage: "FQDN of the remote party"},
					cli.StringFlag{Name: "type", Usage: "Cert type (host or user)"},
				},
				Action: c.getRemoteCerts,
			},
			{
				Name:  "rm",
				Usage: "Remote remote CA from list of trusted certs",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "Certificate id"},
					cli.StringFlag{Name: "fqdn", Usage: "FQDN of the remote party"},
					cli.StringFlag{Name: "type", Usage: "Cert type (host or user)"},
				},
				Action: c.deleteRemoteCert,
			},
		},
	}
}*/

func (cmd *Command) resetHostCA(confirm bool) {
	if !confirm && !cmd.confirm("Reseting private and public keys for Host CA. This will invalidate all signed host certs. Continue?") {
		cmd.printError(fmt.Errorf("aborted by user"))
		return
	}
	if err := cmd.client.ResetHostCA(); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("CA keys have been regenerated")
}

func (cmd *Command) getHostCAPub() {
	key, err := cmd.client.GetHostCAPub()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Host CA Key")
	fmt.Fprintf(cmd.out, string(key))
}

func (cmd *Command) resetUserCA(confirm bool) {
	if !confirm && !cmd.confirm("Reseting private and public keys for User CA. This will invalidate all signed user certs. Continue?") {
		cmd.printError(fmt.Errorf("aborted by user"))
		return
	}
	if err := cmd.client.ResetUserCA(); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("CA keys have been regenerated")
}

func (cmd *Command) getUserCAPub() {
	key, err := cmd.client.GetUserCAPub()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("User CA Key")
	fmt.Fprintf(cmd.out, string(key))
}

func (cmd *Command) upsertRemoteCert(id, fqdn, certType, path string, ttl time.Duration) {
	val, err := cmd.readInput(path)
	if err != nil {
		cmd.printError(err)
		return
	}
	cert := backend.RemoteCert{
		FQDN:  fqdn,
		Type:  certType,
		ID:    id,
		Value: val,
	}
	if err := cmd.client.UpsertRemoteCert(cert, ttl); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Remote cert have been upserted")
}

func (cmd *Command) getRemoteCerts(fqdn, certType string) {
	certs, err := cmd.client.GetRemoteCerts(certType, fqdn)
	if err != nil {
		cmd.printError(err)
		return
	}
	fmt.Fprintf(cmd.out, remoteCertsView(certs))
}

func (cmd *Command) deleteRemoteCert(id, fqdn, certType string) {
	err := cmd.client.DeleteRemoteCert(certType, fqdn, id)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("certificate deleted")
}

func remoteCertsView(certs []backend.RemoteCert) string {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprint(t, "Type\tFQDN\tID\tValue\n")
	if len(certs) == 0 {
		return t.String()
	}
	for _, c := range certs {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", c.Type, c.FQDN, c.ID, string(c.Value))
	}
	return t.String()
}

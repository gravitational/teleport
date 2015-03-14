package command

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/utils"
)

type Command struct {
	url    string
	client *auth.Client
	out    io.Writer
	in     io.Reader
}

func NewCommand() *Command {
	return &Command{
		out: os.Stdout,
		in:  os.Stdin,
	}
}

func (cmd *Command) Run(args []string) error {
	url, args, err := findURL(args)
	if err != nil {
		return err
	}
	cmd.url = url
	cmd.client = auth.NewClient(cmd.url)

	app := cli.NewApp()
	app.Name = "tctl"
	app.Usage = "CLI for key management of teleport SSH cluster"
	app.Flags = flags()

	app.Commands = []cli.Command{
		newHostCACommand(cmd),
		newUserCACommand(cmd),
		newUserCommand(cmd),
	}
	return app.Run(args)
}

func (cmd *Command) readInput(path string) ([]byte, error) {
	if path != "" {
		return utils.ReadPath(path)
	}
	reader := bufio.NewReader(cmd.in)
	return reader.ReadSlice('\n')
}

func (cmd *Command) confirm(message string) bool {
	reader := bufio.NewReader(cmd.in)
	fmt.Fprintf(cmd.out, fmt.Sprintf("%v (Y/N): ", message))
	text, _ := reader.ReadString('\n')
	text = strings.Trim(text, "\n\r\t")
	return text == "Y" || text == "yes" || text == "y"
}

func (cmd *Command) printResult(format string, in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOK(format, fmt.Sprintf("%v", in))
	}
}

func (cmd *Command) printStatus(in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOK("%s", in)
	}
}

func (cmd *Command) printError(err error) {
	fmt.Fprint(cmd.out, goterm.Color(fmt.Sprintf("ERROR: %s", err), goterm.RED)+"\n")
}

func (cmd *Command) printOK(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out,
		goterm.Color(
			fmt.Sprintf("OK: %s\n", fmt.Sprintf(message, params...)), goterm.GREEN)+"\n")
}

func (cmd *Command) printInfo(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out, "INFO: %s\n", fmt.Sprintf(message, params...))
}

// This function extracts url from the command line regardless of it's position
// this is a workaround, as cli libary does not support "superglobal" urls yet.
func findURL(args []string) (string, []string, error) {
	for i, arg := range args {
		if strings.HasPrefix(arg, "--teleport=") || strings.HasPrefix(arg, "-teleport=") {
			out := strings.Split(arg, "=")
			return out[1], cut(i, i+1, args), nil
		} else if strings.HasPrefix(arg, "-teleport") || strings.HasPrefix(arg, "--teleport") {
			// This argument should not be the last one
			if i > len(args)-2 {
				return "", nil, fmt.Errorf("provide a valid URL")
			}
			return args[i+1], cut(i, i+2, args), nil
		}
	}
	return "http://localhost:2023", args, nil
}

func cut(i, j int, args []string) []string {
	s := []string{}
	s = append(s, args[:i]...)
	return append(s, args[j:]...)
}

func flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "auth", Value: DefaultTeleportURL, Usage: "Teleport URL"},
	}
}

const DefaultTeleportURL = "localhost:2023"

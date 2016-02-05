package tool

import (
	"fmt"
	"io/ioutil"
	"log/syslog"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/Sirupsen/logrus"
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
)

// CLI tools by default log into syslog, not stderr
func InitLoggerCLI() {
	hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
	if err != nil {
		panic(err)
	}
	logrus.AddHook(hook)
	// ... and disable its own output:
	logrus.SetOutput(ioutil.Discard)
	logrus.SetLevel(logrus.InfoLevel)
}

// Errorf is useful for CLI apps: it prints an error to stderr and also
// logs it into syslog
func Errorf(fs string, args ...interface{}) {
	logrus.Errorf(fs, args...)
	fmt.Fprintf(os.Stderr, fs+"\n", args...)
}

// InitCmdlineParser configures kingpin command line args parser with
// some defaults common for all Teleport CLI tools
func InitCmdlineParser(appName, appHelp string) (app *kingpin.Application) {
	app = kingpin.New(appName, appHelp)

	// hide "--help" flag
	app.HelpFlag.Hidden()
	app.HelpFlag.NoEnvar()

	// set our own help template
	return app.UsageTemplate(UsageTemplate)
}

// Usage template with compactly formatted commands.
var UsageTemplate = `{{define "FormatCommand"}}\
{{if .FlagSummary}} {{.FlagSummary}}{{end}}\
{{range .Args}} {{if not .Required}}[{{end}}<{{.Name}}>{{if .Value|IsCumulative}}...{{end}}{{if not .Required}}]{{end}}{{end}}\
{{end}}\

{{define "FormatCommands"}}\
{{range .FlattenedCommands}}\
{{if not .Hidden}}\
  {{.FullCommand | printf "%-12s" }}{{if .Default}}*{{end}} {{ .Help }}
{{end}}\
{{end}}\
{{end}}\

{{define "FormatUsage"}}\
{{template "FormatCommand" .}}{{if .Commands}} <command> [<args> ...]{{end}}
{{if .Help}}
{{.Help|Wrap 0}}\
{{end}}\

{{end}}\

{{if .Context.SelectedCommand}}\
usage: {{.App.Name}} {{.Context.SelectedCommand}}{{template "FormatUsage" .Context.SelectedCommand}}
{{else}}\
Usage: {{.App.Name}}{{template "FormatUsage" .App}}
{{end}}\
{{if .Context.Flags}}\
Flags:
{{.Context.Flags|FlagsToTwoColumns|FormatTwoColumns}}
{{end}}\
{{if .Context.Args}}\
Args:
{{.Context.Args|ArgsToTwoColumns|FormatTwoColumns}}
{{end}}\
{{if .Context.SelectedCommand}}\

{{ if .Context.SelectedCommand.Commands}} \
Subcommands:
{{if .Context.SelectedCommand.Commands}}\
{{template "FormatCommands" .Context.SelectedCommand}}
{{end}}\
{{end}}\


{{else if .App.Commands}}\
Commands:
{{template "FormatCommands" .App}}
{{end}}\
`

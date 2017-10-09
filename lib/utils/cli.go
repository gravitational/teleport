/*
Copyright 2016 Gravitational, Inc.

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

package utils

import (
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"strconv"

	"github.com/gravitational/teleport"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

type LoggingPurpose int

const (
	LoggingForDaemon LoggingPurpose = iota
	LoggingForCLI
	LoggingForTests
)

// InitLogger configures the global logger for a given purpose / verbosity level
func InitLogger(purpose LoggingPurpose, level log.Level) {
	log.StandardLogger().Hooks = make(log.LevelHooks)
	formatter := &trace.TextFormatter{}
	formatter.DisableTimestamp = true
	log.SetFormatter(formatter)
	log.SetLevel(level)

	switch purpose {
	case LoggingForCLI:
		SwitchLoggingtoSyslog()
	case LoggingForDaemon:
		log.SetOutput(os.Stderr)
	case LoggingForTests:
		log.SetLevel(level)
		val, _ := strconv.ParseBool(os.Getenv(teleport.VerboseLogsEnvVar))
		if val {
			return
		}
		log.SetLevel(log.WarnLevel)
		log.SetOutput(ioutil.Discard)
	}
}

func InitLoggerForTests() {
	InitLogger(LoggingForTests, log.DebugLevel)
}

// SwitchLoggingtoSyslog tells the logger to send the output to syslog
func SwitchLoggingtoSyslog() {
	log.StandardLogger().Hooks = make(log.LevelHooks)
	hook, err := logrusSyslog.NewSyslogHook("", "", syslog.LOG_WARNING, "")
	if err != nil {
		// syslog not available
		log.SetOutput(os.Stderr)
		log.Warn("syslog not available. reverting to stderr")
	} else {
		// ... and disable stderr:
		log.AddHook(hook)
		log.SetOutput(ioutil.Discard)
	}
}

// FatalError is for CLI front-ends: it detects gravitational/trace debugging
// information, sends it to the logger, strips it off and prints a clean message to stderr
func FatalError(err error) {
	fmt.Fprintln(os.Stderr, UserMessageFromError(err))
	os.Exit(1)
}

// UserMessageFromError returns user friendly error message from error
func UserMessageFromError(err error) string {
	// untrusted cert?
	switch innerError := trace.Unwrap(err).(type) {
	case x509.HostnameError:
		return fmt.Sprintf("Cannot establish https connection to %s:\n%s\n%s\n",
			innerError.Host,
			innerError.Error(),
			"try a different hostname for --proxy or specify --insecure flag if you know what you're doing.")
	case x509.UnknownAuthorityError:
		return `WARNING:

  The proxy you are connecting to has presented a certificate signed by a
  unknown authority. This is most likely due to either being presented
  with a self-signed certificate or the certificate was truly signed by an
  authority not known to the client.

  If you know the certificate is self-signed and would like to ignore this
  error use the --insecure flag.

  If you have your own certificate authority that you would like to use to
  validate the certificate chain presented by the proxy, set the
  SSL_CERT_FILE and SSL_CERT_DIR environment variables respectively and try
  again.

  If you think something malicious may be occurring, contact your Teleport
  system administrator to resolve this issue.
`
	case x509.CertificateInvalidError:
		return fmt.Sprintf(`WARNING:

  The certificate presented by the proxy is invalid: %v.

  Contact your Teleport system administrator to resolve this issue.`, innerError)
	}
	if log.GetLevel() == log.DebugLevel {
		return trace.DebugReport(err)
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// Consolef prints the same message to a 'ui console' (if defined) and also to
// the logger with INFO priority
func Consolef(w io.Writer, msg string, params ...interface{}) {
	msg = fmt.Sprintf(msg, params...)
	if w != nil {
		fmt.Fprintln(w, msg)
	}
	log.Info(msg)
}

// InitCLIParser configures kingpin command line args parser with
// some defaults common for all Teleport CLI tools
func InitCLIParser(appName, appHelp string) (app *kingpin.Application) {
	app = kingpin.New(appName, appHelp)

	// hide "--help" flag
	app.HelpFlag.Hidden()
	app.HelpFlag.NoEnvar()

	// set our own help template
	return app.UsageTemplate(defaultUsageTemplate)
}

// Usage template with compactly formatted commands.
var defaultUsageTemplate = `{{define "FormatCommand"}}\
{{if .FlagSummary}} {{.FlagSummary}}{{end}}\
{{range .Args}} {{if not .Required}}[{{end}}<{{.Name}}>{{if .Value|IsCumulative}}...{{end}}{{if not .Required}}]{{end}}{{end}}\
{{end}}\

{{define "FormatCommands"}}\
{{range .FlattenedCommands}}\
{{if not .Hidden}}\
  {{.FullCommand | printf "%-12s" }}{{if .Default}} (Default){{end}} {{ .Help }}
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
{{.Context.Flags|FlagsToTwoColumnsCompact|FormatTwoColumns}}
{{end}}\
{{if .Context.Args}}\
Args:
{{.Context.Args|ArgsToTwoColumns|FormatTwoColumns}}
{{end}}\
{{if .Context.SelectedCommand}}\

{{ if .Context.SelectedCommand.Commands}}\
Commands:
{{if .Context.SelectedCommand.Commands}}\
{{template "FormatCommands" .Context.SelectedCommand}}
{{end}}\
{{end}}\

{{else if .App.Commands}}\
Commands:
{{template "FormatCommands" .App}}
Try '{{.App.Name}} help [command]' to get help for a given command.
{{end}}\

{{ if .Context.SelectedCommand }}\
{{ range .Context.SelectedCommand.Aliases}}\
{{ . }}
{{end}}\
{{end}}
`

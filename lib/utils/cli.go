/*
Copyright 2016-2021 Gravitational, Inc.

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
	"bytes"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	stdlog "log"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gravitational/teleport"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

type LoggingPurpose int

const (
	LoggingForDaemon LoggingPurpose = iota
	LoggingForCLI
)

// InitLogger configures the global logger for a given purpose / verbosity level
func InitLogger(purpose LoggingPurpose, level log.Level, verbose ...bool) {
	log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	log.SetLevel(level)
	switch purpose {
	case LoggingForCLI:
		// If debug logging was asked for on the CLI, then write logs to stderr.
		// Otherwise discard all logs.
		if level == log.DebugLevel {
			log.SetFormatter(&trace.TextFormatter{
				DisableTimestamp: true,
				EnableColors:     trace.IsTerminal(os.Stderr),
			})
			log.SetOutput(os.Stderr)
		} else {
			log.SetOutput(ioutil.Discard)
		}
	case LoggingForDaemon:
		log.SetFormatter(&trace.TextFormatter{
			DisableTimestamp: true,
			EnableColors:     trace.IsTerminal(os.Stderr),
		})
		log.SetOutput(os.Stderr)
	}
}

// InitLoggerForTests initializes the standard logger for tests.
func InitLoggerForTests() {
	// Parse flags to check testing.Verbose().
	flag.Parse()

	logger := log.StandardLogger()
	logger.ReplaceHooks(make(log.LevelHooks))
	logger.SetFormatter(&trace.TextFormatter{})
	logger.SetLevel(log.DebugLevel)
	logger.SetOutput(os.Stderr)
	if testing.Verbose() {
		return
	}
	logger.SetLevel(log.WarnLevel)
	logger.SetOutput(ioutil.Discard)
}

// NewLoggerForTests creates a new logger for test environment
func NewLoggerForTests() *log.Logger {
	logger := log.New()
	logger.ReplaceHooks(make(log.LevelHooks))
	logger.SetFormatter(&trace.TextFormatter{})
	logger.SetLevel(log.DebugLevel)
	logger.SetOutput(os.Stderr)
	return logger
}

// WrapLogger wraps an existing logger entry and returns
// an value satisfying the Logger interface
func WrapLogger(logger *log.Entry) Logger {
	return &logWrapper{Entry: logger}
}

// NewLogger creates a new empty logger
func NewLogger() *log.Logger {
	logger := log.New()
	logger.SetFormatter(&trace.TextFormatter{
		DisableTimestamp: true,
		EnableColors:     trace.IsTerminal(os.Stderr),
	})
	return logger
}

// Logger describes a logger value
type Logger interface {
	log.FieldLogger
	// GetLevel specifies the level at which this logger
	// value is logging
	GetLevel() log.Level
	// SetLevel sets the logger's level to the specified value
	SetLevel(level log.Level)
}

// FatalError is for CLI front-ends: it detects gravitational/trace debugging
// information, sends it to the logger, strips it off and prints a clean message to stderr
func FatalError(err error) {
	fmt.Fprintln(os.Stderr, UserMessageFromError(err))
	os.Exit(1)
}

// GetIterations provides a simple way to add iterations to the test
// by setting environment variable "ITERATIONS", by default it returns 1
func GetIterations() int {
	out := os.Getenv(teleport.IterationsEnvVar)
	if out == "" {
		return 1
	}
	iter, err := strconv.Atoi(out)
	if err != nil {
		panic(err)
	}
	log.Debugf("Starting tests with %v iterations.", iter)
	return iter
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
		var buf bytes.Buffer
		fmt.Fprintln(&buf, Color(Red, "ERROR:"))
		// If the error is a trace error, check if it has a user message embedded in
		// it, if it does, print it, otherwise escape and print the original error.
		if er, ok := err.(*trace.TraceErr); ok {
			for _, message := range er.Messages {
				fmt.Fprintln(&buf, "\t"+EscapeControl(message))
			}
			fmt.Fprintln(&buf, "\t"+EscapeControl(trace.Unwrap(er).Error()))
		} else {
			fmt.Fprintln(&buf, EscapeControl(err.Error()))
		}
		return buf.String()
	}
	return ""
}

const (
	// Red is an escape code for red terminal color
	Red = 31
	// Yellow is an escape code for yellow terminal color
	Yellow = 33
	// Blue is an escape code for blue terminal color
	Blue = 36
	// Gray is an escape code for gray terminal color
	Gray = 37
)

// Color formats the string in a terminal escape color
func Color(color int, v interface{}) string {
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", color, v)
}

// Consolef prints the same message to a 'ui console' (if defined) and also to
// the logger with INFO priority
func Consolef(w io.Writer, log log.FieldLogger, component, msg string, params ...interface{}) {
	msg = fmt.Sprintf(msg, params...)
	log.Info(msg)
	if w != nil {
		component := strings.ToUpper(component)
		// 13 is the length of "[KUBERNETES]", which is the longest component
		// name prefix we have *today*. Use a Max function here to avoid
		// negative spacing, in case we add longer component names.
		spacing := int(math.Max(float64(12-len(component)), 0))
		fmt.Fprintf(w, "[%v]%v %v\n", strings.ToUpper(component), strings.Repeat(" ", spacing), msg)
	}
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

// EscapeControl escapes all ANSI escape sequences from string and returns a
// string that is safe to print on the CLI. This is to ensure that malicious
// servers can not hide output. For more details, see:
//   * https://sintonen.fi/advisories/scp-client-multiple-vulnerabilities.txt
func EscapeControl(s string) string {
	if needsQuoting(s) {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// NewStdlogger creates a new stdlib logger that uses the specified leveled logger
// for output and the given component as a logging prefix.
func NewStdlogger(logger LeveledOutputFunc, component string) *stdlog.Logger {
	return stdlog.New(&stdlogAdapter{
		log: logger,
	}, component, stdlog.LstdFlags)
}

// Write writes the specified buffer p to the underlying leveled logger.
// Implements io.Writer
func (r *stdlogAdapter) Write(p []byte) (n int, err error) {
	r.log(string(p))
	return len(p), nil
}

// stdlogAdapter is an io.Writer that writes into an instance
// of logrus.Logger
type stdlogAdapter struct {
	log LeveledOutputFunc
}

// LeveledOutputFunc describes a function that emits given
// arguments at a specific level to an underlying logger
type LeveledOutputFunc func(args ...interface{})

// GetLevel returns the level of the underlying logger
func (r *logWrapper) GetLevel() logrus.Level {
	return r.Entry.Logger.GetLevel()
}

// SetLevel sets the logging level to the given value
func (r *logWrapper) SetLevel(level logrus.Level) {
	r.Entry.Logger.SetLevel(level)
}

// logWrapper wraps a log entry.
// Implements Logger
type logWrapper struct {
	*logrus.Entry
}

// needsQuoting returns true if any non-printable characters are found.
func needsQuoting(text string) bool {
	for _, r := range text {
		if !strconv.IsPrint(r) {
			return true
		}
	}
	return false
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
Aliases:
{{ range .Context.SelectedCommand.Aliases}}\
{{ . }}
{{end}}\
{{end}}
`

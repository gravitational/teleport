// Package testlog provides custom loggers for use in tests.
package testlog

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

// FailureOnly returns a logger that only prints the logs to STDERR when the
// test fails.
func FailureOnly(t *testing.T) *logrus.Entry {
	// Collect all the output in buf.
	buf := &bytes.Buffer{}
	log := logrus.New()
	log.Out = buf

	// Register a cleanup callback which prints buf iff t has failed.
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		fmt.Fprintln(os.Stderr, buf.String())
	})

	return logrus.NewEntry(log)
}

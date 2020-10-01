/*
Copyright 2020 Gravitational, Inc.

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

// Package testlog provides custom loggers for use in tests.
package testlog

import (
	"fmt"
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/sirupsen/logrus"
)

// FailureOnly returns a logger that only prints the logs to STDERR when the
// test fails.
func FailureOnly(t *testing.T) *logrus.Entry {
	// Collect all the output in buf.
	buf := utils.NewSyncBuffer()
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

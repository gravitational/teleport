/*
Copyright 2021 Gravitational, Inc.

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

package types

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// UTC converts time to UTC timezone
func UTC(t *time.Time) {
	if t == nil {
		return
	}

	if t.IsZero() {
		// to fix issue with timezones for tests
		*t = time.Time{}
		return
	}
	*t = t.UTC()
}

// HumanTimeFormatString is a human readable date formatting
const HumanTimeFormatString = "Mon Jan _2 15:04 UTC"

// HumanTimeFormat formats time as recognized by humans
func HumanTimeFormat(d time.Time) string {
	return d.Format(HumanTimeFormatString)
}

// ParseBool parses string as boolean value,
// returns error in case if value is not recognized
func ParseBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "yes", "yeah", "y", "true", "1", "on":
		return true, nil
	case "no", "nope", "n", "false", "0", "off":
		return false, nil
	default:
		return false, trace.BadParameter("unsupported value: %q", value)
	}
}

// InitLoggerForTests initializes the standard logger for tests with verbosity
func InitLoggerForTests(verbose bool) {
	logger := log.StandardLogger()
	logger.ReplaceHooks(make(log.LevelHooks))
	logger.SetFormatter(&trace.TextFormatter{})
	logger.SetLevel(log.DebugLevel)
	logger.SetOutput(os.Stderr)
	if verbose {
		return
	}
	logger.SetLevel(log.WarnLevel)
	logger.SetOutput(ioutil.Discard)
}

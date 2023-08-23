// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lib

import (
	"os"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Bail exits with nonzero exit code and prints an error to a log.
func Bail(err error) {
	if agg, ok := trace.Unwrap(err).(trace.Aggregate); ok {
		for i, err := range agg.Errors() {
			log.WithError(err).Errorf("Terminating with fatal error [%d]...", i+1)
		}
	} else {
		log.WithError(err).Error("Terminating with fatal error...")
	}
	os.Exit(1)
}

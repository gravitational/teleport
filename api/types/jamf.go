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

package types

import (
	"net/url"
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

const (
	// JamfOnMissingNOOP is the textual representation for the NOOP on_missing
	// action.
	JamfOnMissingNoop = "NOOP"
	// JamfOnMissingDelete is the textual representation for the DELETE on_missing
	// action.
	JamfOnMissingDelete = "DELETE"
)

// JamfOnMissingActions is a slice of all textual on_missing representations,
// excluding the empty string.
var JamfOnMissingActions = []string{
	JamfOnMissingNoop,
	JamfOnMissingDelete,
}

// ValidateJamfSpecV1 validates a [JamfSpecV1] instance.
func ValidateJamfSpecV1(s *JamfSpecV1) error {
	if s == nil {
		return trace.BadParameter("spec required")
	}

	switch u, err := url.Parse(s.ApiEndpoint); {
	case err != nil:
		return trace.BadParameter("invalid API endpoint: %v", err)
	case u.Host == "":
		return trace.BadParameter("invalid API endpoint: missing hostname")
	}

	for i, e := range s.Inventory {
		switch {
		case e == nil:
			return trace.BadParameter("inventory entry #%v is nil", i)
		case e.OnMissing != "" && !slices.Contains(JamfOnMissingActions, e.OnMissing):
			return trace.BadParameter(
				"inventory[%v]: invalid on_missing action %q (expect empty or one of [%v])",
				i, e.OnMissing, strings.Join(JamfOnMissingActions, ","))
		}

		syncPartial := e.SyncPeriodPartial
		syncFull := e.SyncPeriodFull
		if syncFull > 0 && syncPartial >= syncFull {
			return trace.BadParameter("inventory[%v]: sync_period_partial is greater or equal to sync_period_full, partial syncs will never happen", i)
		}
	}

	return nil
}

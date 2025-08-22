/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package bot_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/bot"
)

func TestCredentialLifetimeValidate(t *testing.T) {
	testCases := map[string]struct {
		cfg     bot.CredentialLifetime
		oneShot bool
		error   string
	}{
		"partial config": {
			cfg:   bot.CredentialLifetime{TTL: 1 * time.Minute},
			error: "credential_ttl and renewal_interval must both be specified if either is",
		},
		"negative TTL": {
			cfg:   bot.CredentialLifetime{TTL: -time.Minute, RenewalInterval: time.Minute},
			error: "credential_ttl must be positive",
		},
		"negative renewal interval": {
			cfg:   bot.CredentialLifetime{TTL: time.Minute, RenewalInterval: -time.Minute},
			error: "renewal_interval must be positive",
		},
		"TTL less than renewal interval": {
			cfg:   bot.CredentialLifetime{TTL: time.Minute, RenewalInterval: 2 * time.Minute},
			error: "TTL is shorter than the renewal interval",
		},
		"TTL less than renewal interval (one-shot)": {
			cfg:     bot.CredentialLifetime{TTL: time.Minute, RenewalInterval: 2 * time.Minute},
			oneShot: true,
			error:   "",
		},
		"TTL too long": {
			cfg:   bot.CredentialLifetime{TTL: defaults.MaxRenewableCertTTL * 2, RenewalInterval: time.Minute},
			error: "TTL exceeds the maximum TTL allowed",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.Validate(tc.oneShot)

			if tc.error == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.error)
			}
		})
	}
}

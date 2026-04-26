/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package winpki

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/testhelpers/mtls"
	"github.com/gravitational/teleport/lib/srv/desktop/ldaptest"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestLDAPReadWithFilter(t *testing.T) {
	base := "OU=Windows,DC=example,DC=com"
	host1 := ldaptest.NewComputerEntry("host1", base)
	host2 := ldaptest.NewComputerEntry("host2", base)

	mtls := mtls.NewConfig(t)

	srv, err := ldaptest.NewServer(mtls.ServerTLS)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	srv.SetEntries(base, host1, host2)

	client, err := DialLDAP(t.Context(),
		&LDAPConfig{
			Addr:     srv.Addr,
			Domain:   "example.com",
			Username: "test-user",
			SID:      "test-sid",
			Logger:   slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{})),
		},
		mtls.ClientTLS,
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	entries, err := client.ReadWithFilter(base, "(objectClass=computer)", []string{"*"})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "CN=host1,OU=Windows,DC=example,DC=com", entries[0].DN)
	require.Equal(t, "CN=host2,OU=Windows,DC=example,DC=com", entries[1].DN)
}

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package accessrequest

import (
	"fmt"
	"net/url"

	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

func ExampleMsgFields_roles() {
	id := "00000000-0000-0000-0000-000000000000"
	req := plugindata.AccessRequestData{
		User:          "example@goteleport.com",
		Roles:         []string{"foo", "bar"},
		Resources:     []string{"/example.teleport.sh/node/0000"},
		RequestReason: "test",
	}
	cluster := "example.teleport.sh"
	webProxyURL := &url.URL{
		Scheme:  "https",
		Host:    "example.teleport.sh",
		RawPath: "web/requests/00000000-0000-0000-0000-000000000000",
	}

	msg := MsgFields(id, req, cluster, webProxyURL)
	fmt.Println(msg)

	// Output: *ID*: 00000000-0000-0000-0000-000000000000
	// *Cluster*: example.teleport.sh
	// *User*: example@goteleport.com
	// *Role(s)*: `bar,foo`
	// *Resource(s)*: `/example.teleport.sh/node/0000`
	// *Reason*: ```
	// test```
	// *Link*: https://example.teleport.sh/web/requests/00000000-0000-0000-0000-000000000000
}

func ExampleMsgFields_logins() {
	id := "00000000-0000-0000-0000-000000000000"
	req := plugindata.AccessRequestData{
		User:  "example@goteleport.com",
		Roles: []string{"admin", "foo", "bar", "dev"},
		LoginsByRole: map[string][]string{
			"admin": {"foo", "bar", "root"},
			"foo":   {"foo"},
			"bar":   {"bar"},
			"dev":   {},
		},
		Resources:     []string{"/example.teleport.sh/node/0000"},
		RequestReason: "test",
	}
	cluster := "example.teleport.sh"
	webProxyURL := &url.URL{
		Scheme:  "https",
		Host:    "example.teleport.sh",
		RawPath: "web/requests/00000000-0000-0000-0000-000000000000",
	}

	msg := MsgFields(id, req, cluster, webProxyURL)
	fmt.Println(msg)

	// Output: *ID*: 00000000-0000-0000-0000-000000000000
	// *Cluster*: example.teleport.sh
	// *User*: example@goteleport.com
	// *Role*: `admin` *Login(s)*: `bar, foo, root`
	// *Role*: `bar` *Login(s)*: `bar`
	// *Role*: `dev` *Login(s)*: `-`
	// *Role*: `foo` *Login(s)*: `foo`
	// *Resource(s)*: `/example.teleport.sh/node/0000`
	// *Reason*: ```
	// test```
	// *Link*: https://example.teleport.sh/web/requests/00000000-0000-0000-0000-000000000000
}

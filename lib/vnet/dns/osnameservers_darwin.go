// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dns

import (
	"bufio"
	"context"
	"log/slog"
	"net/netip"
	"os"
	"strings"

	"github.com/gravitational/trace"
)

const (
	confFilePath = "/etc/resolv.conf"
)

// platformLoadUpstreamNameservers reads the OS DNS nameservers found in
// /etc/resolv.conf. The comments in that file make it clear it is not actually
// consulted for DNS hostname resolution, but MacOS seems to keep it up to date
// with the current default nameservers as configured for the OS, and it is the
// easiest place to read them. Eventually we should probably use a better
// method, but for now this works.
func platformLoadUpstreamNameservers(ctx context.Context) ([]string, error) {
	f, err := os.Open(confFilePath)
	if err != nil {
		return nil, trace.Wrap(err, "opening %s", confFilePath)
	}
	defer f.Close()

	var nameservers []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "nameserver ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		address := fields[1]

		ip, err := netip.ParseAddr(address)
		if err != nil {
			slog.DebugContext(ctx, "Skipping invalid IP", "ip", address, "error", err)
			continue
		}

		nameservers = append(nameservers, withDNSPort(ip))
	}

	slog.DebugContext(ctx, "Loaded host upstream nameservers.", "nameservers", nameservers, "config_file", confFilePath)
	return nameservers, nil
}

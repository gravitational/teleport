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

//go:build darwin
// +build darwin

package dns

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	confFilePath = "/etc/resolv.conf"
)

type OSUpstreamNameserverSource struct {
	ttlCache *utils.FnCache
}

func NewOSUpstreamNameserverSource() (*OSUpstreamNameserverSource, error) {
	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: 10 * time.Second,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &OSUpstreamNameserverSource{
		ttlCache: ttlCache,
	}, nil
}

func (s *OSUpstreamNameserverSource) UpstreamNameservers(ctx context.Context) ([]string, error) {
	return utils.FnCacheGet(ctx, s.ttlCache, 0, s.upstreamNameservers)
}

func (s *OSUpstreamNameserverSource) upstreamNameservers(ctx context.Context) ([]string, error) {
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

		ip := net.ParseIP(address)
		if ip == nil {
			slog.DebugContext(ctx, "Skipping invalid IP", "ip", address)
			continue
		}

		// Add port 53 suffix, the only port supported on MacOS.
		var nameserver string
		switch {
		case ip.To4() != nil:
			nameserver = address + ":53"
		case ip.To16() != nil:
			nameserver = "[" + address + "]:53"
		default:
			continue
		}
		nameservers = append(nameservers, nameserver)
	}

	slog.DebugContext(ctx, "Loaded host upstream nameservers.", "nameservers", nameservers, "source", confFilePath)
	return nameservers, nil
}

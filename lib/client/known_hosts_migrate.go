/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package client

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
)

// knownHostEntry is a parsed entry from a Teleport/OpenSSH known_hosts file,
// used as part of the migration/pruning process to make Teleport's known_hosts
// compatible with OpenSSH.
type knownHostEntry struct {
	raw     string
	marker  string
	hosts   []string
	pubKey  ssh.PublicKey
	comment string
}

// parseKnownHost parses a single line of a known hosts file into a struct.
func parseKnownHost(raw string) (*knownHostEntry, error) {
	// Due to the lack of first-class tuples, we'll need to wrap this in a
	// struct to avoid re-parsing lines constantly. We'll also keep the input
	// text to preserve formatting for all passed-through entries.
	marker, hosts, pubKey, comment, _, err := ssh.ParseKnownHosts([]byte(raw))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &knownHostEntry{
		raw:     raw,
		marker:  marker,
		hosts:   hosts,
		pubKey:  pubKey,
		comment: comment,
	}, nil
}

// isOldStyleHostsEntry determines if a particular known host entry is explicitly
// formatted as an old-style entry. Only old-style entries are candidates for
// pruning; all others are passed through untouched.
func isOldStyleHostsEntry(entry *knownHostEntry) bool {
	if entry.marker != "" {
		return false
	}

	if len(entry.hosts) != 1 {
		return false
	}

	if entry.comment != "" {
		return false
	}

	return true
}

// canPruneOldHostsEntry determines if a particular old-style hosts entry has an
// equivalent new-style entry and can thus be pruned. Note that this will panic
// if `oldEntry` does not contain at least one host; `isOldStyleHostsEntry`
// validates this.
func canPruneOldHostsEntry(oldEntry *knownHostEntry, newEntries []*knownHostEntry) bool {
	// Note: Per sshd's documentation, it is valid (though not recommended) for
	// repeated/overlapping entries to exist for a given host; as such, it's
	// only safe to prune an old entry when both the (*.)hostname and public key
	// match.

	// The new-style entries prepend `*.`, so we'll add that upfront.
	oldHost := "*." + oldEntry.hosts[0]

	// We'll need to marshal the keys so we can compare them properly.
	oldKey := oldEntry.pubKey.Marshal()

	for _, newEntry := range newEntries {
		if oldEntry.pubKey.Type() != newEntry.pubKey.Type() {
			continue
		}

		newKey := newEntry.pubKey.Marshal()
		if !bytes.Equal(oldKey, newKey) {
			continue
		}

		for _, newHost := range newEntry.hosts {
			if newHost == oldHost {
				return true
			}
		}
	}

	return false
}

// pruneOldHostKeys removes all old-style host keys for which a new-style
// duplicate entry exists. This may modify order of host keys, but will not
// change their content.
func pruneOldHostKeys(output []string) []string {
	log := slog.With(teleport.ComponentKey, teleport.ComponentMigrate)

	var (
		oldEntries   = make([]*knownHostEntry, 0)
		newEntries   = make([]*knownHostEntry, 0)
		prunedOutput = make([]string, 0)
	)

	// First, categorize all existing known hosts entries.
	for i, line := range output {
		parsed, err := parseKnownHost(line)
		if err != nil {
			// If the line isn't parseable, pass it through.
			log.DebugContext(context.Background(), "Unable to parse known host, skipping",
				"invalid_line_number", i+1,
				"error", err,
			)
			prunedOutput = append(prunedOutput, line)
			continue
		}

		if isOldStyleHostsEntry(parsed) {
			// Only old-style entries are candidates for removal.
			oldEntries = append(oldEntries, parsed)
		} else {
			// Everything else is passed through as-is...
			prunedOutput = append(prunedOutput, line)

			// ...but only new-style entries are candidates for comparison.
			if parsed.marker == "cert-authority" {
				newEntries = append(newEntries, parsed)
			}
		}
	}

	// Next, for each old-style entry, determine if an existing new-style entry
	// exists. If not, pass it through.
	for _, entry := range oldEntries {
		if canPruneOldHostsEntry(entry, newEntries) {
			log.DebugContext(context.Background(), "Pruning old known_hosts entry for host", "host", entry.hosts[0])
		} else {
			prunedOutput = append(prunedOutput, entry.raw)
		}
	}

	return prunedOutput
}

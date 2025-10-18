// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package joinutils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// GlobMatch performs simple a simple glob-style match test on a string.
// - '*' matches zero or more characters.
// - '?' matches any single character.
// It returns true if a match is detected.
func GlobMatch(pattern, str string) (bool, error) {
	pattern = regexp.QuoteMeta(pattern)
	pattern = strings.ReplaceAll(pattern, `\*`, ".*")
	pattern = strings.ReplaceAll(pattern, `\?`, ".")
	pattern = "^" + pattern + "$"
	matched, err := regexp.MatchString(pattern, str)
	return matched, trace.Wrap(err)
}

// GenerateChallenge generates a crypto-random challenge with length random
// bytes and encodes it to base64.
func GenerateChallenge(encoding *base64.Encoding, length int) (string, error) {
	// read crypto-random bytes to generate the challenge
	challengeRawBytes := make([]byte, length)
	if _, err := rand.Read(challengeRawBytes); err != nil {
		return "", trace.Wrap(err)
	}

	// encode the challenge to base64 so it can be sent over HTTP
	return encoding.EncodeToString(challengeRawBytes), nil
}

// RawJoinAttrsToStruct converts raw join attributes into a struct suitable for
// logging or audit events.
func RawJoinAttrsToStruct(in any) (*apievents.Struct, error) {
	if in == nil {
		return nil, nil
	}
	attrBytes, err := json.Marshal(in)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling join attributes")
	}
	out := &apievents.Struct{}
	if err := out.UnmarshalJSON(attrBytes); err != nil {
		return nil, trace.Wrap(err, "unmarshaling join attributes")
	}
	return out, nil
}

// SanitizeUntrustedString sanitizes an untrusted string from a joining client
// for inclusion in a log message or audit event.
func SanitizeUntrustedString(in string) string {
	var out strings.Builder
	const maxLen = 512
	wasSpace := false
	for _, r := range in {
		// Break once the output reaches the max length.
		if out.Len() >= maxLen {
			break
		}

		// Coalesce runs of spaces to a single space.
		if unicode.IsSpace(r) {
			if !wasSpace {
				out.WriteRune(' ')
			}
			wasSpace = true
			continue
		}
		wasSpace = false

		// Strip all non-print characters.
		if !unicode.IsPrint(r) {
			out.WriteRune('_')
			continue
		}

		// Allow all letters and numbers.
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			continue
		}

		// Whitelist allowed symbols. Avoid allowing links or quotes.
		switch r {
		case '.', ',', ':', ';', '-', '+', '@':
			out.WriteRune(r)
		default:
			out.WriteRune('_')
		}
	}
	return out.String()
}

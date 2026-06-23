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

package resourcematcher

import (
	"github.com/gravitational/trace"
)

// Result is the outcome of Match: whether the tree matched and the captures it
// bound. A non-match is the zero value, so there is no nil to guard.
type Result struct {
	// Matched reports whether the tree matched the path.
	Matched bool
	// Captures holds the segments the matcher's capture nodes bound. It is nil
	// on a non-match and may be an empty map on a match that bound nothing.
	Captures map[string]string
}

// Match tokenizes path, applies the encoded-char gate the opts allow, and walks
// the matcher tree. It is the standalone parallel to the path.match predicate:
// a path carrying an encoded char that no allow_encoded option admits does not
// match (fail closed), and a path the tokenizer rejects does not match. Use
// Tokenize plus Eval directly to tell a tokenize error from a plain no-match.
func Match(root *Node, path string, opts ...Option) Result {
	tokens, err := Tokenize(path)
	if err != nil {
		return Result{}
	}
	if encodedBlocked(tokens, opts) {
		return Result{}
	}
	ok, caps := Eval(tokens, root)
	return Result{Matched: ok, Captures: caps}
}

// AllowEncoded builds the option that opts a Match (or path.match) into the
// named encoded chars. A variadic string is natural in Go; the predicate
// surface uses set("/") instead. Only "/" is supported today.
func AllowEncoded(chars ...string) (Option, error) {
	if err := validateEncodedChars(chars); err != nil {
		return Option{}, trace.Wrap(err)
	}
	return Option{allowEncoded: chars}, nil
}

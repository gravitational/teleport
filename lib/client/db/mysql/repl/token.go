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

package repl

// token is a lexical token.
type token struct {
	// kind is a token kind enum.
	kind tokenKind
	// text is the text that makes up the token.
	text string
}

type tokenKind int

const (
	// EOF indicates end of token stream
	tokenEOF tokenKind = iota

	// whitespace
	tokenSpace

	// escape
	tokenBackslash // \

	// quotation syntax
	tokenSingleQuote // '
	tokenDoubleQuote // "
	tokenBacktick    // `

	// single-line comments
	tokenSingleComment // -- or #

	// multi-line comment open/close
	tokenOpenComment  // /*
	tokenCloseComment // */

	// misc text that isn't special to us
	tokenText
)

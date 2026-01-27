// Copyright 2025 Gravitational, Inc.
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

package pkixname

import (
	"bytes"
	"container/list"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	oidRegexp      = regexp.MustCompile(`^\d+(\.\d+)*$`)
	attrTypeRegexp = regexp.MustCompile(`^[A-Za-z]([A-Za-z0-9-])*$`)
)

// ParseDistinguishedName parses an RFC 2253-like distinguished name parser.
//
// For example: "C=US,O=Teleport,CN=Teleport CA".
//
// Deviations in relation to the RFC:
//   - Common OIDs are not supported: use "C" instead of 2.5.4.6, "O" instead of
//     2.5.4.10, etc.
//   - Hexstrings are not supported (ie, "#1234ABCD"). Custom OIDs values must
//     be strings.
//   - Attribute types may not be prefixed with "oid." or "OID.".
//   - Escaped characters are limited to specials and the space character (' ').
//     No other escapes are allowed, including hex escaping.
//   - Multi-valued RDNs are only allowed if all values refer to the same
//     attributeType.
//   - The only character interpreted as whitespace is the space character
//     (' ').
//
// Reference: https://www.rfc-editor.org/rfc/rfc2253.
func ParseDistinguishedName(dn string) (*pkix.Name, error) {
	const maxDNLength = 4096 // arbitrary-ish upper value
	switch {
	case dn == "": // Early exit.
		return &pkix.Name{}, nil
	case len(dn) > maxDNLength:
		return nil, errors.New("distinguished name too large, refusing to parse")
	}

	tokens, err := tokenize(dn)
	if err != nil {
		return nil, err
	}

	dst := &pkix.Name{}
	if tokens.Len() == 0 {
		return dst, nil
	}
	if err := parseRDNSequence(dst, *tokens); err != nil {
		return nil, fmt.Errorf("malformed RDNs: %w", err)
	}
	return dst, nil
}

// parseRDNSequence parses a RelativeDistinguishedName sequence, ie, a sequence
// of AttributeTypeAndValue separated by commas or pluses.
func parseRDNSequence(dst *pkix.Name, tokens tokenList) error {
	seenAttrs := make(map[string]struct{})
	markAttr := func(attr string) error {
		if _, ok := seenAttrs[attr]; ok {
			return fmt.Errorf("repeated attributeType %q, remaining tokens: %s", attr, tokens)
		}
		seenAttrs[attr] = struct{}{}
		return nil
	}

	prevAttr, err := parseATV(dst, tokens)
	if err != nil {
		return err
	}
	_ = markAttr(prevAttr)

	for {
		tok, ok := tokens.Peek()
		if !ok {
			return nil // end
		}

		switch tok.kind {
		case tokenPlus:
			tokens.PopSilently()

			// Validate that prevAttr == current attr.
			// If `!ok` just keep going and parseATV() will fail.
			if nextTok, ok := tokens.Peek(); ok &&
				nextTok.kind == tokenAttrType &&
				nextTok.value != prevAttr {
				return fmt.Errorf(
					"multi-valued RDN must refer to the same attribute, but found %q instead of %q, remaining tokens: %s",
					prevAttr,
					nextTok.value,
					tokens,
				)
			}

			prevAttr, err = parseATV(dst, tokens)
			if err != nil {
				return err
			}

		case tokenComma:
			tokens.PopSilently()
			prevAttr, err = parseATV(dst, tokens)
			if err != nil {
				return err
			}
			if err := markAttr(prevAttr); err != nil {
				return err
			}

		default:
			// Force an error.
			return requireTokenKind(tokenComma, tok, tokens)
		}
	}
}

// parseATV parses an AttributeTypeAndValue.
//
// Eg: "CN" EQUAL "Llama CA".
func parseATV(dst *pkix.Name, tokens tokenList) (attr string, _ error) {
	t1, t2, t3, ok := tokens.Peek3()
	if !ok {
		return "", fmt.Errorf(
			"not enough tokens to parse AttributeTypeValue, remaining tokens: %s",
			tokens,
		)
	}
	if err := requireTokenKind(tokenAttrType, t1, tokens); err != nil {
		return "", err
	}
	if err := requireTokenKind(tokenEqual, t2, tokens); err != nil {
		return "", err
	}
	if err := requireTokenKind(tokenString, t3, tokens); err != nil {
		return "", err
	}

	// Pop tokens before returning. We retain the tokens up until the end so
	// eventual errors include them in the message.
	defer func() {
		tokens.PopSilently()
		tokens.PopSilently()
		tokens.PopSilently()
	}()

	attr = t1.value
	value := t3.value

	// Parse as OID?
	if oidRegexp.MatchString(attr) {
		return attr, parseOIDExtraName(dst, attr, value)
	}
	// Verify attributeType character set.
	if !attrTypeRegexp.MatchString(attr) {
		return "", fmt.Errorf("invalid attributeType (bad character set): %q", attr)
	}

	switch attr {
	case "SERIALNUMBER":
		dst.SerialNumber = value
	case "CN":
		dst.CommonName = value
	case "OU":
		dst.OrganizationalUnit = append(dst.OrganizationalUnit, value)
	case "O":
		dst.Organization = append(dst.Organization, value)
	case "POSTALCODE":
		dst.PostalCode = append(dst.PostalCode, value)
	case "STREET":
		dst.StreetAddress = append(dst.StreetAddress, value)
	case "L":
		dst.Locality = append(dst.Locality, value)
	case "ST":
		dst.Province = append(dst.Province, value)
	case "C":
		dst.Country = append(dst.Country, value)
	default:
		return "", fmt.Errorf("unknown attributeType %q, remaining tokens: %s", attr, tokens)
	}
	return attr, nil
}

func parseOIDExtraName(dst *pkix.Name, attr, value string) error {
	parts := strings.Split(attr, ".")
	oid := make(asn1.ObjectIdentifier, 0, len(parts))
	for _, val := range parts {
		num, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf(
				"cannot parse OID component %q as int, OID=%q: %w", val, attr, err)
		}
		oid = append(oid, num)
	}

	dst.ExtraNames = append(dst.ExtraNames, pkix.AttributeTypeAndValue{
		Type:  oid,
		Value: value,
	})
	return nil
}

func requireTokenKind(wantKind tokenKind, tok *token, tokens tokenList) error {
	if tok.kind == wantKind {
		return nil
	}
	return fmt.Errorf(
		"found %s instead of %s, remaining tokens: %s",
		tok.kind,
		wantKind,
		tokens,
	)
}

type tokenKind int

const (
	tokenAttrType = iota + 1
	tokenString
	tokenEqual
	tokenPlus
	tokenComma
)

func (k tokenKind) String() string {
	switch k {
	case tokenAttrType:
		return "ATTR"
	case tokenString:
		return "STRING"
	case tokenEqual:
		return "EQUAL"
	case tokenPlus:
		return "PLUS"
	case tokenComma:
		return "COMMA"
	default:
		return "UNKNOWN"
	}
}

type token struct {
	kind  tokenKind
	value string
}

func (t token) String() string {
	switch t.kind {
	case tokenAttrType:
		return t.value
	case tokenString:
		return fmt.Sprintf("%q", t.value)
	default:
		return t.kind.String()
	}
}

type tokenizeState int

const (
	// initial state, ignores whitespaces.
	// Transitions to attrType.
	// Is a valid final state.
	tokenizeStateInit = iota
	// Wants an attributeType, ignores whitespaces.
	// Transitions to attrType.
	tokenizeStateNameComponent
	// attributeType parsing started.
	// Transitions to attrTypeEnd or stringStart.
	tokenizeStateAttrType
	// attributeType parsing ended, ignores whitespace.
	// Transitions to stringStart.
	tokenizeStateAttrTypeEnd
	// Wants string, ignores whitespaces.
	// Transitions to string (without consuming the rune), stringQuote or
	// nameComponent.
	// Is a valid final state.
	tokenizeStateStringStart
	// string parsing started.
	// Transitions to stringEnd, stringEscape or nameComponent.
	// Is a valid final state.
	tokenizeStateString
	// string parsing found a whitespace. Buffers whitespaces.
	// Transitions to string (without consuming the rune) or nameComponent.
	// Is a valid final state.
	tokenizeStateStringEnd
	// string escape parsing stated (found '\\'), wants the escaped rune.
	// Transitions back to string or stringQuote.
	tokenizeStateStringEscape
	// Quoted string parsing started.
	// Transitions to stringEscape or stringQuoteEnd.
	tokenizeStateStringQuote
	// Quoted string parsing ended. Ignores whitespace.
	// Transitions to nameComponent.
	// Is a valid final state.
	tokenizeStateStringQuoteEnd
)

type tokenList struct {
	list *list.List
}

func (l tokenList) Len() int {
	return l.list.Len()
}

func (l tokenList) Peek() (*token, bool) {
	if l.list.Len() == 0 {
		return nil, false
	}

	e := l.list.Front()
	return e.Value.(*token), true
}

func (l tokenList) Peek3() (_, _, _ *token, _ bool) {
	if l.list.Len() < 3 {
		return nil, nil, nil, false
	}

	e1 := l.list.Front()
	e2 := e1.Next()
	e3 := e2.Next()
	return e1.Value.(*token), e2.Value.(*token), e3.Value.(*token), true
}

func (l tokenList) PopSilently() {
	l.list.Remove(l.list.Front())
}

func (l tokenList) PushValue(k tokenKind, v string) {
	l.list.PushBack(&token{kind: k, value: v})
}

func (l tokenList) Push(k tokenKind) {
	l.PushValue(k, "")
}

func (l tokenList) String() string {
	if l.list.Len() == 0 {
		return ""
	}

	buf := &bytes.Buffer{}
	root := l.list.Front()
	for e := root; e != nil; e = e.Next() {
		if e != root {
			buf.WriteRune(' ')
		}
		buf.WriteString(e.Value.(*token).String())
	}

	return buf.String()
}

// tokenize parses a DN string into a sequence of tokens.
// It handles whitespaces, quotes, escapes and guarantees a correct sequence of
// tokens: ATTR EQUAL STRING, optionally followed by a PLUS/COMMA and another
// sequence.
//
// An empty string, or a string composed only of whitespace, results in an empty
// list of tokens.
//
// attributeType values are not validated. They are guaranteed to be formed by
// attributeType or OID characters, but may be invalid according to either type.
// It's up to callers to narrow down and validate them.
//
// https://www.rfc-editor.org/rfc/rfc2253#section-3.
func tokenize(dn string) (*tokenList, error) {
	tokens := &tokenList{
		list: list.New(),
	}
	emit := func(k tokenKind) {
		tokens.Push(k)
	}

	buf := &bytes.Buffer{}
	trailingSpaceBuf := &bytes.Buffer{}
	emitBuffer := func(k tokenKind) {
		val := buf.String()
		buf.Reset()
		tokens.PushValue(k, val)
	}

	errTrace := func(pos int) string {
		end := min(pos+10, len(dn)) // arbitrary "forward" point into dn.
		return fmt.Sprintf("pos=%d, substring %q", pos, dn[pos:end])
	}

	var state, prevState tokenizeState
	escapeStart := func() {
		prevState = state
		state = tokenizeStateStringEscape
	}
	escapeEnd := func() {
		state = prevState
	}

	// Used only with '+', ',' and ';' runes.
	transitionToNameComponent := func(r rune) {
		switch r {
		case '+':
			emit(tokenPlus)
		case ',', ';':
			emit(tokenComma)
		default:
			panic(fmt.Sprintf("cannot transition to name-component from rune %q, state %v", r, state))
		}
		state = tokenizeStateNameComponent
	}

	for pos, r := range dn {
		// Skip whitespace in these states.
		if r == ' ' &&
			(state == tokenizeStateInit ||
				state == tokenizeStateNameComponent ||
				state == tokenizeStateAttrTypeEnd ||
				state == tokenizeStateStringStart ||
				state == tokenizeStateStringQuoteEnd) {
			continue
		}

		// string start/end handling.
		// This happens early because we sometimes transition to "string" without
		// consuming the rune.
		switch state {
		case tokenizeStateStringStart:
			switch r {
			case '+', ',', ';':
				// Empty strings are valid.
				emitBuffer(tokenString)
				transitionToNameComponent(r)
				continue
			case '#':
				return nil, fmt.Errorf("hexstring not supported: %s", errTrace(pos))
			case '"':
				// Note that a quoted string may be empty.
				state = tokenizeStateStringQuote
				continue
			default:
				state = tokenizeStateString
				// Rune not consumed.
			}

		case tokenizeStateStringEnd:
			switch r {
			case ' ':
				trailingSpaceBuf.WriteRune(r)
				continue
			case '+', ',', ';':
				trailingSpaceBuf.Reset() // whitespace discarded.
				emitBuffer(tokenString)
				transitionToNameComponent(r)
				continue
			default:
				trailingSpaceBuf.WriteTo(buf) // whitespace copied back.
				trailingSpaceBuf.Reset()
				state = tokenizeStateString
				// Rune not consumed.
			}
		}

		switch state {
		case tokenizeStateInit, tokenizeStateNameComponent:
			switch {
			case isAttrType(r):
				state = tokenizeStateAttrType
				buf.WriteRune(r)
			default:
				return nil, fmt.Errorf("want attributeType, found %q: %s", r, errTrace(pos))
			}

		case tokenizeStateAttrType:
			switch {
			case isAttrType(r):
				buf.WriteRune(r)
			case r == '=':
				emitBuffer(tokenAttrType)
				emit(tokenEqual)
				state = tokenizeStateStringStart
			case r == ' ':
				emitBuffer(tokenAttrType)
				state = tokenizeStateAttrTypeEnd
			default:
				return nil, fmt.Errorf("want attributeType or '=', found %q: %s", r, errTrace(pos))
			}

		case tokenizeStateAttrTypeEnd:
			switch r {
			case '=':
				emit(tokenEqual)
				state = tokenizeStateStringStart
			default:
				return nil, fmt.Errorf("want '=' attributeValue, found %q: %s", r, errTrace(pos))
			}

		case tokenizeStateString:
			switch r {
			case '+', ',', ';':
				emitBuffer(tokenString)
				transitionToNameComponent(r)
			case '\\':
				escapeStart()
			case '<', '>', '"':
				// We could '<' and '>', but let's be strict.
				return nil, fmt.Errorf("special character %q not quoted: %s", r, errTrace(pos))
			case ' ':
				trailingSpaceBuf.WriteRune(r)
				state = tokenizeStateStringEnd
			case '=', '#': // Go does this.
				// NOT OK per RFC, should be escaped.
				fallthrough
			default:
				buf.WriteRune(r)
			}

		case tokenizeStateStringEscape:
			switch r {
			case ' ': // Go does this.
				// OK per RFC and allows the "\\ " trick.
				fallthrough
			case ',', '=', '+', '<', '>', '#', ';', '\\', '"':
				buf.WriteRune(r)
				escapeEnd()
			default:
				return nil, fmt.Errorf("unexpected escaped character %q: %s", r, errTrace(pos))
			}

		case tokenizeStateStringQuote:
			switch r {
			case '\\':
				escapeStart()
			case '"':
				emitBuffer(tokenString)
				state = tokenizeStateStringQuoteEnd
			default:
				buf.WriteRune(r)
			}

		case tokenizeStateStringQuoteEnd:
			switch r {
			case '+', ',', ';':
				transitionToNameComponent(r)
			default:
				return nil, fmt.Errorf("want '+' or ',', found %q: %s", r, errTrace(pos))
			}
		}
	}

	// Input ended, check the final state.
	switch state {
	case tokenizeStateInit:
		// OK.
	case tokenizeStateNameComponent:
		return nil, fmt.Errorf("want attributeType, found EOF")
	case tokenizeStateAttrType:
		return nil, fmt.Errorf("want attributeType or '=', found EOF")
	case tokenizeStateAttrTypeEnd:
		return nil, fmt.Errorf("want '=' attributeValue, found EOF")
	case tokenizeStateStringStart, tokenizeStateString, tokenizeStateStringEnd:
		// OK.
		emitBuffer(tokenString)
	case tokenizeStateStringEscape:
		return nil, fmt.Errorf("want escaped character, found EOF")
	case tokenizeStateStringQuote:
		return nil, fmt.Errorf("want closing quote, found EOF")
	case tokenizeStateStringQuoteEnd:
		// OK.
	default:
		// This should not be reached. All states are handled above.
		return nil, fmt.Errorf("found EOF (state=%d)", state)
	}

	return tokens, nil
}

func isAttrType(r rune) bool {
	return r >= 'A' && r <= 'Z' ||
		r >= 'a' && r <= 'z' ||
		r >= '0' && r <= '9' ||
		r == '-' ||
		r == '.'
}

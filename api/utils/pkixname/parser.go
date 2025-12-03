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

var oidRegexp = regexp.MustCompile(`^\d+(\.\d+)*$`)
var attrTypeRegexp = regexp.MustCompile(`^[A-Za-z]([A-Za-z0-9-])*$`)

func ParseDistinguishedName(s string) (*pkix.Name, error) {
	const maxDNLength = 4096 // arbitrary-ish upper value
	if len(s) > maxDNLength {
		return nil, errors.New("string too large, refusing to parse")
	}

	tokens, err := tokenize(s)
	if err != nil {
		return nil, err
	}

	dst := &pkix.Name{}
	if tokens == nil || tokens.Len() == 0 {
		return dst, nil
	}

	if err := parseRDNSequence(dst, *tokens); err != nil {
		return nil, fmt.Errorf("malformed RDNs: %w", err)
	}
	return dst, nil
}

// parseRDNSequence parses a RelativeDistinguishedName sequence, ie, a sequence
// of AttributeTypeAndValue separated by commas.
func parseRDNSequence(dst *pkix.Name, tokens tokenList) error {
	if tokens.Len() == 0 {
		return nil
	}

	// Sequence must begin with an ATV.
	prevAttr, err := parseATV(dst, tokens)
	if err != nil {
		return err
	}

	for {
		tok, ok := tokens.Peek()
		if !ok {
			return nil // end
		}

		switch tok.kind {
		case tokenPlus:
			tokens.PopSilently()

			// Validate that prevAttr == nextAttr.
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

		default:
			// Force an error.
			return requireTokenKind(tokenComma, "", tok, tokens)
		}
	}
}

// parseATV parses an AttributeTypeAndValue.
//
// eg: "CN=Llama CA"
func parseATV(dst *pkix.Name, tokens tokenList) (attr string, _ error) {
	t1, t2, t3, ok := tokens.Peek3()
	if !ok {
		return "", fmt.Errorf(
			"not enough tokens to parse AttributeTypeValue, remaining tokens: %s",
			tokens,
		)
	}
	if err := requireTokenKind(tokenAttrType, attrName, t1, tokens); err != nil {
		return "", err
	}
	if err := requireTokenKind(tokenEqual, "", t2, tokens); err != nil {
		return "", err
	}
	if err := requireTokenKind(tokenString, valueName, t3, tokens); err != nil {
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

	// Note: We don't handle common OIDs!
	if oidRegexp.MatchString(attr) {
		parts := strings.Split(attr, ".")
		oid := make(asn1.ObjectIdentifier, 0, len(parts))
		for _, val := range parts {
			num, err := strconv.Atoi(val)
			if err != nil {
				return "", fmt.Errorf("failed to parse attributeType %q as OID: %w", attr, err)
			}
			oid = append(oid, num)
		}

		dst.ExtraNames = append(dst.ExtraNames, pkix.AttributeTypeAndValue{
			Type:  oid,
			Value: value,
		})
		return
	}

	if !attrTypeRegexp.MatchString(attr) {
		return "", fmt.Errorf("invalid attributeType: %q", attr)
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
		return "", fmt.Errorf("failed to parse attribute %q as OID or common attribute, reamining tokens: %s", attr, tokens)
	}

	return attr, nil
}

func requireTokenKind(wantKind tokenKind, wantName string, tok *token, tokens tokenList) error {
	if tok.kind == wantKind {
		return nil
	}

	if wantName == "" {
		wantName = wantKind.String()
	}

	return fmt.Errorf(
		"found %s instead of %s, remaining tokens: %s",
		tok.kind,
		wantName,
		tokens,
	)
}

type tokenKind int

const (
	tokenEmpty tokenKind = iota
	tokenAttrType
	tokenString
	tokenEqual
	tokenPlus
	tokenComma
)

const (
	attrName  = "ATTR"
	valueName = "VALUE"
)

func (k tokenKind) String() string {
	switch k {
	case tokenEmpty:
		return "EMPTY"
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
	// initial state.
	// Is a valid final state.
	tokenizeStateInit = iota
	// Wants an attributeType.
	// Transitions to attrType.
	tokenizeStateNameComponent
	// attributeType parsing started.
	// Transitions to stringStart.
	tokenizeStateAttrType
	// Wants string, ignores whitespaces.
	// Transitions to string or stringQuote
	tokenizeStateStringStart
	// string parsing started.
	// Transitions to stringEnd, stringEscape or nameComponent.
	// Is a valid final state.
	tokenizeStateString
	// string parsing found a whitespace. Buffers whitespaces in a separate buffer.
	// Transitions to string (non-space) or nameComponent.
	// Is a valid final state.
	tokenizeStateStringEnd
	// string escape parsing stated (found '\\').
	// Transitions to string or stringQuote.
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

func tokenize(s string) (*tokenList, error) {
	var tokens *tokenList
	emitValue := func(k tokenKind, v string) {
		if tokens == nil {
			tokens = &tokenList{
				list: list.New(),
			}
		}
		tokens.list.PushBack(&token{kind: k, value: v})
	}
	emit := func(k tokenKind) {
		emitValue(k, "")
	}

	buf := &bytes.Buffer{}
	trailingSpaceBuf := &bytes.Buffer{}
	buffer := func(r rune) {
		buf.WriteRune(r)
	}
	emitBuffer := func(k tokenKind) {
		if buf.Len() == 0 {
			return
		}

		val := buf.String()
		buf.Reset()

		emitValue(k, val)
	}

	errTrace := func(pos int) string {
		end := min(pos+10, len(s))
		return fmt.Sprintf("pos=%d, substring %q", pos, s[pos:end])
	}

	var state, prevState tokenizeState
	escapeStart := func() {
		prevState = state
		state = tokenizeStateStringEscape
	}
	escapeEnd := func() {
		state = prevState
	}

	for pos, r := range s {
		// Skip whitespace in these states.
		if r == ' ' &&
			(state == tokenizeStateInit ||
				state == tokenizeStateNameComponent ||
				state == tokenizeStateStringStart ||
				state == tokenizeStateStringQuoteEnd) {
			continue
		}

		// "string" start/end handling.
		// This happens early because we sometimes transition to "string" and let
		// it handle the rune.
		switch state {
		case tokenizeStateStringStart:
			switch r {
			case '"':
				state = tokenizeStateStringQuote
				continue
			default:
				state = tokenizeStateString
			}

		case tokenizeStateStringEnd:
			switch r {
			case ' ':
				trailingSpaceBuf.WriteRune(r)
				continue
			case '+':
				emitBuffer(tokenString)
				emit(tokenPlus)
				trailingSpaceBuf.Reset()
				state = tokenizeStateNameComponent
				continue
			case ',', ';':
				emitBuffer(tokenString)
				emit(tokenComma)
				trailingSpaceBuf.Reset()
				state = tokenizeStateNameComponent
				continue
			default:
				// Push spaces into buf and transition back to "string".
				trailingSpaceBuf.WriteTo(buf)
				trailingSpaceBuf.Reset()
				state = tokenizeStateString
			}
		}

		switch state {
		case tokenizeStateInit, tokenizeStateNameComponent:
			switch {
			case isAttr(r):
				state = tokenizeStateAttrType
				buffer(r)
			default:
				return nil, fmt.Errorf("want attributeType, found %q: %s", r, errTrace(pos))
			}

		case tokenizeStateAttrType:
			switch {
			case isAttr(r):
				buffer(r)
			case r == '=':
				emitBuffer(tokenAttrType)
				emit(tokenEqual)
				state = tokenizeStateStringStart
			default:
				return nil, fmt.Errorf("want attributeType or '=', found %q: %s", r, errTrace(pos))
			}

		case tokenizeStateString:
			switch r {
			case '+':
				emitBuffer(tokenString)
				emit(tokenPlus)
				state = tokenizeStateNameComponent

			case ',', ';':
				emitBuffer(tokenString)
				emit(tokenComma)
				state = tokenizeStateNameComponent

			case '\\':
				escapeStart()

			case '"':
				return nil, fmt.Errorf("want unquoted string, found %q: %s", r, errTrace(pos))

			case '#':
				return nil, fmt.Errorf("hexstring not supported: %s", errTrace(pos))

			case '=', '<', '>':
				// We _could_ allow these, but let's be strict.
				return nil, fmt.Errorf("special character %q not quoted: %s", r, errTrace(pos))

			case ' ':
				trailingSpaceBuf.WriteRune(r)
				state = tokenizeStateStringEnd

			default:
				buffer(r)
			}

		case tokenizeStateStringEscape:
			switch r {
			case ',', '=', '+', '<', '>', '#', ';', '\\', '"':
				buffer(r)
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
				buffer(r)
			}

		case tokenizeStateStringQuoteEnd:
			switch r {
			case '+':
				emit(tokenPlus)
				state = tokenizeStateNameComponent
			case ',', ';':
				emit(tokenComma)
				state = tokenizeStateNameComponent
			default:
				return nil, fmt.Errorf("want '+' or ',', found %q: %s", r, errTrace(pos))
			}
		}
	}

	switch state {
	case tokenizeStateInit:
		return tokens, nil
	case tokenizeStateNameComponent:
		return nil, fmt.Errorf("want attributeType, found EOF")
	case tokenizeStateAttrType:
		return nil, fmt.Errorf("want attributeType or '=', found EOF")
	case tokenizeStateStringStart:
		return nil, fmt.Errorf("want attributeValue, found EOF")
	case tokenizeStateString, tokenizeStateStringEnd, tokenizeStateStringQuoteEnd:
		// OK to end on a well-formed string.
		emitBuffer(tokenString)
	case tokenizeStateStringEscape:
		return nil, fmt.Errorf("want escaped character, found EOF")
	case tokenizeStateStringQuote:
		return nil, fmt.Errorf("want closing quote, found EOF")
	default:
		// This should not be reached. All states are handled above.
		return nil, fmt.Errorf("found EOF (state=%d)", state)
	}

	return tokens, nil
}

func isAttr(r rune) bool {
	return r >= 'A' && r <= 'Z' ||
		r >= 'a' && r <= 'z' ||
		r >= '0' && r <= '9' ||
		r == '-' ||
		r == '.'
}

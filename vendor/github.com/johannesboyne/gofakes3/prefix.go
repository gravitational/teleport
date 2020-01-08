package gofakes3

import (
	"fmt"
	"net/url"
	"strings"
)

type Prefix struct {
	HasPrefix bool
	Prefix    string

	HasDelimiter bool
	Delimiter    string
}

func prefixFromQuery(query url.Values) Prefix {
	prefix := Prefix{
		Prefix:    query.Get("prefix"),
		Delimiter: query.Get("delimiter"),
	}
	_, prefix.HasPrefix = query["prefix"]
	_, prefix.HasDelimiter = query["delimiter"]
	return prefix
}

func NewPrefix(prefix, delim *string) (p Prefix) {
	if prefix != nil {
		p.HasPrefix, p.Prefix = true, *prefix
	}
	if delim != nil {
		p.HasDelimiter, p.Delimiter = true, *delim
	}
	return p
}

func NewFolderPrefix(prefix string) (p Prefix) {
	p.HasPrefix, p.Prefix = true, prefix
	p.HasDelimiter, p.Delimiter = true, "/"
	return p
}

// FilePrefix returns the path portion, then the remaining portion of the
// Prefix if the Delimiter is "/". If the Delimiter is not set, or not "/",
// ok will be false.
//
// For example:
//	/foo/bar/  : path: /foo/bar  remaining: ""
//	/foo/bar/b : path: /foo/bar  remaining: "b"
//	/foo/bar   : path: /foo      remaining: "bar"
//
func (p Prefix) FilePrefix() (path, remaining string, ok bool) {
	if !p.HasPrefix || !p.HasDelimiter || p.Delimiter != "/" {
		return "", "", ok
	}

	idx := strings.LastIndexByte(p.Prefix, '/')
	if idx < 0 {
		return "", p.Prefix, true
	} else {
		return p.Prefix[:idx], p.Prefix[idx+1:], true
	}
}

// PrefixMatch checks whether key starts with prefix. If the prefix does not
// match, nil is returned.
//
// It is a best-effort attempt to implement the prefix/delimiter matching found
// in S3.
//
// To check whether the key belongs in Contents or CommonPrefixes, compare the
// result to key.
//
func (p Prefix) Match(key string, match *PrefixMatch) (ok bool) {
	if !p.HasPrefix && !p.HasDelimiter {
		// If there is no prefix in the search, the match is the prefix:
		if match != nil {
			*match = PrefixMatch{Key: key, MatchedPart: key}
		}
		return true
	}

	if !p.HasDelimiter {
		// If the request does not contain a delimiter, prefix matching is a
		// simple string prefix:
		if strings.HasPrefix(key, p.Prefix) {
			if match != nil {
				*match = PrefixMatch{Key: key, MatchedPart: p.Prefix}
			}
			return true
		}
		return false
	}

	// Delimited + Prefix matches, for example:
	//	 $ aws s3 ls s3://my-bucket/
	//	                            PRE AWSLogs/
	//	 $ aws s3 ls s3://my-bucket/AWSLogs
	//	                            PRE AWSLogs/
	//	 $ aws s3 ls s3://my-bucket/AWSLogs/
	//	                            PRE 260839334643/
	//	 $ aws s3 ls s3://my-bucket/AWSLogs/2608
	//	                            PRE 260839334643/

	keyParts := strings.Split(strings.TrimLeft(key, p.Delimiter), p.Delimiter)
	preParts := strings.Split(strings.TrimLeft(p.Prefix, p.Delimiter), p.Delimiter)

	if len(keyParts) < len(preParts) {
		return false
	}

	// If the key exactly matches the prefix, but only up to a delimiter,
	// AWS appends the delimiter to the result:
	//	 $ aws s3 ls s3://my-bucket/AWSLogs
	//	                            PRE AWSLogs/
	appendDelim := len(keyParts) != len(preParts)
	matched := 0

	last := len(preParts) - 1
	for i := 0; i < len(preParts); i++ {
		if i == last {
			if !strings.HasPrefix(keyParts[i], preParts[i]) {
				return false
			}

		} else {
			if keyParts[i] != preParts[i] {
				return false
			}
		}
		matched++
	}

	if matched == 0 {
		return false
	}

	out := strings.Join(keyParts[:matched], p.Delimiter)
	if appendDelim {
		out += p.Delimiter
	}

	if match != nil {
		*match = PrefixMatch{Key: key, CommonPrefix: out != key, MatchedPart: out}
	}
	return true
}

func (p Prefix) String() string {
	if p.HasDelimiter {
		return fmt.Sprintf("prefix:%q, delim:%q", p.Prefix, p.Delimiter)
	} else {
		return fmt.Sprintf("prefix:%q", p.Prefix)
	}
}

type PrefixMatch struct {
	// Input key passed to PrefixMatch.
	Key string

	// CommonPrefix indicates whether this key should be returned in the bucket
	// contents or the common prefixes part of the "list bucket" response.
	CommonPrefix bool

	// The longest matched part of the key.
	MatchedPart string
}

func (match *PrefixMatch) AsCommonPrefix() CommonPrefix {
	return CommonPrefix{Prefix: match.MatchedPart}
}

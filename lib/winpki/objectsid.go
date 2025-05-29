// Modified from https://github.com/bwmarrin/go-objectsid
// BSD 2-Clause License

// Copyright (c) 2019, bwmarrin
// All rights reserved.

// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:

// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.

// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.

// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package winpki

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
)

// adSID is an Active Directory ObjectSID
type adSID struct {
	RevisionLevel     int
	SubAuthorityCount int
	Authority         int
	SubAuthorities    []int
	RelativeID        *int
}

// String value is: S-Revision-Authority-SubAuthority[n]...
func (sid adSID) String() string {
	s := fmt.Sprintf("S-%d-%d", sid.RevisionLevel, sid.Authority)
	for _, v := range sid.SubAuthorities {
		s += fmt.Sprintf("-%d", v)
	}
	return s
}

// The binary data is in the form:
// byte[0] - revision level
// byte[1] - count of sub-authorities
// byte[2-7] - 48-bit authority (big-endian)
// and then count x 32-bit sub authorities (little-endian)
//
// Based on code from here: http://www.adamretter.org.uk/blog/entries/active-directory-ldap-users-primary-group.xml
func decodeADSID(b []byte) (adSID, error) {
	len := len(b)
	if len < 8 {
		return adSID{}, trace.BadParameter("AD SID response was too short to decode")
	}
	var sid adSID

	sid.RevisionLevel = int(b[0])
	sid.SubAuthorityCount = int(b[1])

	// Get the 48-bit, big-endian authority
	for i := 2; i <= 7; i++ {
		sid.Authority = sid.Authority | int(b[i])<<(8*(5-(i-2)))
	}

	// Get the SubAuthorityCount number of 32-bit, little-endian sub authorities
	var offset = 8
	var size = 4
	for i := 0; i < sid.SubAuthorityCount; i++ {
		var subAuthority int
		for k := 0; k < size; k++ {
			index := offset + k
			if index >= len {
				return adSID{}, trace.BadParameter("AD SID response was too short to decode")
			}
			subAuthority = subAuthority | (int(b[index]))<<(8*k)
		}
		sid.SubAuthorities = append(sid.SubAuthorities, subAuthority)
		offset += size
	}

	return sid, nil
}

// ADSIDStringFromLDAPEntry extracts the objectSid attribute from an
// ldap.Entry and returns its string representation.
func ADSIDStringFromLDAPEntry(entry *ldap.Entry) (string, error) {
	bytes := entry.GetRawAttributeValue(AttrObjectSid)
	if len(bytes) == 0 {
		return "", trace.Errorf("failed to find %v", AttrObjectSid)
	}
	sid, err := decodeADSID(bytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return sid.String(), nil
}

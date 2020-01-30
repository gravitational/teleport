/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"fmt"

	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

type AccessRequestSuite struct {
}

var _ = Suite(&AccessRequestSuite{})
var _ = fmt.Printf

func (s *AccessRequestSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

// TestRequestMarshaling verifies that marshaling/unmarshaling access requests
// works as expected (failures likely indicate a problem with json schema).
func (s *AccessRequestSuite) TestRequestMarshaling(c *C) {
	req1, err := NewAccessRequest("some-user", "role-1", "role-2")
	c.Assert(err, IsNil)

	marshaled, err := GetAccessRequestMarshaler().MarshalAccessRequest(req1)
	c.Assert(err, IsNil)

	req2, err := GetAccessRequestMarshaler().UnmarshalAccessRequest(marshaled)
	c.Assert(err, IsNil)

	if !req1.Equals(req2) {
		c.Errorf("unexpected inequality %+v <---> %+v", req1, req2)
	}
}

// TestPluginDataExpectations verifies the correct behavior of the `Expect` mapping.
// Update operations which include an `Expect` mapping should not succeed unless
// all expectations match (e.g. `{"foo":"bar","spam":""}` matches the state where
// key `foo` has value `bar` and key `spam` does not exist).
func (s *AccessRequestSuite) TestPluginDataExpectations(c *C) {
	const rname = "my-resource"
	const pname = "my-plugin"
	data, err := NewPluginData(rname, KindAccessRequest)
	c.Assert(err, IsNil)

	// Set two keys, expecting them to be unset.
	err = data.Update(PluginDataUpdateParams{
		Kind:     KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"hello": "world",
			"spam":  "eggs",
		},
		Expect: map[string]string{
			"hello": "",
			"spam":  "",
		},
	})
	c.Assert(err, IsNil)

	// Expect a value which does not exist.
	err = data.Update(PluginDataUpdateParams{
		Kind:     KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"should": "fail",
		},
		Expect: map[string]string{
			"missing": "key",
		},
	})
	fixtures.ExpectCompareFailed(c, err)

	// Expect a value to not exist when it does exist.
	err = data.Update(PluginDataUpdateParams{
		Kind:     KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"should": "fail",
		},
		Expect: map[string]string{
			"hello": "world",
			"spam":  "",
		},
	})
	fixtures.ExpectCompareFailed(c, err)

	// Expect the correct state, updating one key and removing another.
	err = data.Update(PluginDataUpdateParams{
		Kind:     KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"hello": "there",
			"spam":  "",
		},
		Expect: map[string]string{
			"hello": "world",
			"spam":  "eggs",
		},
	})
	c.Assert(err, IsNil)

	// Expect the new updated state.
	err = data.Update(PluginDataUpdateParams{
		Kind:     KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"should": "succeed",
		},
		Expect: map[string]string{
			"hello": "there",
			"spam":  "",
		},
	})
	c.Assert(err, IsNil)
}

// TestPluginDataFilterMatching verifies the expected matching behavior for PluginDataFilter
func (s *AccessRequestSuite) TestPluginDataFilterMatching(c *C) {
	const rname = "my-resource"
	const pname = "my-plugin"
	data, err := NewPluginData(rname, KindAccessRequest)
	c.Assert(err, IsNil)

	var f PluginDataFilter

	// Filter for a different resource
	f.Resource = "other-resource"
	c.Assert(f.Match(data), Equals, false)

	// Filter for the same resource
	f.Resource = rname
	c.Assert(f.Match(data), Equals, true)

	// Filter for a plugin which does not have data yet
	f.Plugin = pname
	c.Assert(f.Match(data), Equals, false)

	// Add some data
	err = data.Update(PluginDataUpdateParams{
		Kind:     KindAccessRequest,
		Resource: rname,
		Plugin:   pname,
		Set: map[string]string{
			"spam": "eggs",
		},
	})
	// Filter again now that data exists
	c.Assert(err, IsNil)
	c.Assert(f.Match(data), Equals, true)
}

// TestRequestFilterMatching verifies expected matching behavior for AccessRequestFilter.
func (s *AccessRequestSuite) TestRequestFilterMatching(c *C) {
	reqA, err := NewAccessRequest("alice", "role-a")
	c.Assert(err, IsNil)

	reqB, err := NewAccessRequest("bob", "role-b")
	c.Assert(err, IsNil)

	testCases := []struct {
		user   string
		id     string
		matchA bool
		matchB bool
	}{
		{"", "", true, true},
		{"alice", "", true, false},
		{"", reqA.GetName(), true, false},
		{"bob", reqA.GetName(), false, false},
		{"carol", "", false, false},
	}
	for _, tc := range testCases {
		m := AccessRequestFilter{
			User: tc.user,
			ID:   tc.id,
		}
		if m.Match(reqA) != tc.matchA {
			c.Errorf("bad filter behavior (a) %+v", tc)
		}
		if m.Match(reqB) != tc.matchB {
			c.Errorf("bad filter behavior (b) %+v", tc)
		}
	}
}

// TestRequestFilterConversion verifies that filters convert to and from
// maps correctly.
func (s *AccessRequestSuite) TestRequestFilterConversion(c *C) {
	testCases := []struct {
		f AccessRequestFilter
		m map[string]string
	}{
		{
			AccessRequestFilter{User: "alice", ID: "foo", State: RequestState_PENDING},
			map[string]string{"user": "alice", "id": "foo", "state": "PENDING"},
		},
		{
			AccessRequestFilter{User: "bob"},
			map[string]string{"user": "bob"},
		},
		{
			AccessRequestFilter{},
			map[string]string{},
		},
	}
	for _, tc := range testCases {

		if m := tc.f.IntoMap(); !utils.StringMapsEqual(m, tc.m) {
			c.Errorf("bad map encoding: expected %+v, got %+v", tc.m, m)
		}
		var f AccessRequestFilter
		if err := f.FromMap(tc.m); err != nil {
			c.Errorf("failed to parse %+v: %s", tc.m, err)
		}
		if !f.Equals(tc.f) {
			c.Errorf("bad map decoding: expected %+v, got %+v", tc.f, f)
		}
	}
	badMaps := []map[string]string{
		{"food": "carrots"},
		{"state": "homesick"},
	}
	for _, m := range badMaps {
		var f AccessRequestFilter
		c.Assert(f.FromMap(m), NotNil)
	}
}

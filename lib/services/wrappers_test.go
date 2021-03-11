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
	"encoding/hex"
	"testing"

	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/api/types/wrappers"
)

type WrappersSuite struct{}

var _ = check.Suite(&WrappersSuite{})

func TestWrappers(t *testing.T) { check.TestingT(t) }

func (w *WrappersSuite) TestUnmarshalBackwards(c *check.C) {
	var traits wrappers.Traits

	// Attempt to unmarshal protobuf encoded data.
	protoBytes := "0a120a066c6f67696e7312080a06666f6f6261720a150a116b756265726e657465735f67726f7570731200"
	data, err := hex.DecodeString(protoBytes)
	c.Assert(err, check.IsNil)
	err = wrappers.UnmarshalTraits(data, &traits)
	c.Assert(err, check.IsNil)
	c.Assert(traits["logins"], check.DeepEquals, []string{"foobar"})

	// Attempt to unmarshal JSON encoded data.
	jsonBytes := `{"logins": ["foobar"]}`
	err = wrappers.UnmarshalTraits([]byte(jsonBytes), &traits)
	c.Assert(err, check.IsNil)
	c.Assert(traits["logins"], check.DeepEquals, []string{"foobar"})
}

/*
Copyright 2018 Gravitational, Inc.

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

package utils

import (
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

type AnonymizerSuite struct{}

var _ = check.Suite(&AnonymizerSuite{})

func (s *AnonymizerSuite) TestHMACAnonymizer(c *check.C) {
	a, err := NewHMACAnonymizer(" ")
	c.Assert(err, check.FitsTypeOf, trace.BadParameter(""))
	c.Assert(a, check.IsNil)

	a, err = NewHMACAnonymizer("key")
	c.Assert(err, check.IsNil)
	c.Assert(a, check.NotNil)

	data := "secret"
	result := a.Anonymize([]byte(data))
	c.Assert(result, check.Not(check.Equals), "")
	c.Assert(result, check.Not(check.Equals), data)
}

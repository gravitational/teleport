/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
package asciitable

*/
package asciitable

import (
	"testing"

	"gopkg.in/check.v1"
)

// bootstrap check
func TestAsciiTable(t *testing.T) { check.TestingT(t) }

type TableTestSuite struct {
}

var _ = check.Suite(&TableTestSuite{})

const fullTable = `Name          Motto                            Age  
------------- -------------------------------- ---- 
Joe Forrester Trains are much better than cars 40   
Jesus         Read the bible                   2018 
`

const headlessTable = `one two 
1   2   
`

func (s *TableTestSuite) TestFullTable(c *check.C) {
	t := MakeTable([]string{"Name", "Motto", "Age"})
	t.AddRow([]string{"Joe Forrester", "Trains are much better than cars", "40"})
	t.AddRow([]string{"Jesus", "Read the bible", "2018"})

	c.Assert(t.AsBuffer().String(), check.Equals, fullTable)
}

func (s *TableTestSuite) TestHeadlessTable(c *check.C) {
	t := MakeHeadlessTable(2)
	t.AddRow([]string{"one", "two", "three"})
	t.AddRow([]string{"1", "2", "3"})

	// the table shall have no header and also the 3rd column must be chopped off
	c.Assert(t.AsBuffer().String(), check.Equals, headlessTable)
}

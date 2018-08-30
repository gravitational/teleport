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

package asciitable

import (
	"fmt"
)

func ExampleMakeTable() {
	// Create a table with three column headers.
	t := MakeTable([]string{"Token", "Type", "Expiry Time (UTC)"})

	// Add in multiple rows.
	t.AddRow([]string{"b53bd9d3e04add33ac53edae1a2b3d4f", "auth", "30 Aug 18 23:31 UTC"})
	t.AddRow([]string{"5ecde0ca17824454b21937109df2c2b5", "node", "30 Aug 18 23:31 UTC"})
	t.AddRow([]string{"9333929146c08928a36466aea12df963", "trusted_cluster", "30 Aug 18 23:33 UTC"})

	// Write the table to stdout.
	fmt.Println(t.AsBuffer().String())
}

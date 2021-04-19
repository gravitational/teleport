/*
Copyright 2017-2021 Gravitational, Inc.

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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const fullTable = `Name          Motto                            Age  
------------- -------------------------------- ---- 
Joe Forrester Trains are much better than cars 40   
Jesus         Read the bible                   2018 
`

const headlessTable = `one  two  
1    2    
`

const truncatedTable = `Name          Motto                            Age   
------------- -------------------------------- ----- 
Joe Forrester Trains are much better th... [*] 40    
Jesus         Read the bible                   fo... 
X             yyyyyyyyyyyyyyyyyyyyyyyyy... [*]       

[*] Full motto was truncated, use the "tctl motto get" subcommand to view full motto.
`

func TestFullTable(t *testing.T) {
	table := MakeTable([]string{"Name", "Motto", "Age"})
	table.AddRow([]string{"Joe Forrester", "Trains are much better than cars", "40"})
	table.AddRow([]string{"Jesus", "Read the bible", "2018"})

	require.Equal(t, fullTable, table.AsBuffer().String())
}

func TestHeadlessTable(t *testing.T) {
	table := MakeHeadlessTable(2)
	table.AddRow([]string{"one", "two", "three"})
	table.AddRow([]string{"1", "2", "3"})

	// The table shall have no header and also the 3rd column must be chopped off.
	require.Equal(t, headlessTable, table.AsBuffer().String())
}

func TestTruncatedTable(t *testing.T) {
	table := MakeTable([]string{"Name"})
	table.AddColumn(Column{
		Title:         "Motto",
		MaxCellLength: 25,
		FootnoteLabel: "[*]",
	})
	table.AddColumn(Column{
		Title:         "Age",
		MaxCellLength: 2,
	})
	table.AddFootnote(
		"[*]",
		`Full motto was truncated, use the "tctl motto get" subcommand to view full motto.`,
	)
	table.AddRow([]string{"Joe Forrester", "Trains are much better than cars", "40"})
	table.AddRow([]string{"Jesus", "Read the bible", "for ever and ever"})
	table.AddRow([]string{"X", strings.Repeat("y", 26), ""})

	require.Equal(t, truncatedTable, table.AsBuffer().String())
}

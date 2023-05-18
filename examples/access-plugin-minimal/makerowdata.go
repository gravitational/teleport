// Copyright 2023 Gravitational, Inc
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

package main

import (
	"fmt"
	"strings"

	sheets "google.golang.org/api/sheets/v4"

	"github.com/gravitational/teleport/api/types"
)

func stringPtr(s string) *string { return &s }

func (g *googleSheetsPlugin) makeRowData(ar types.AccessRequest) *sheets.RowData {
	requestState, ok := requestStates[ar.GetState()]

	// Could not find a state, but this is still a valid Access Request
	if !ok {
		requestState = requestStates[types.RequestState_NONE]
	}

	viewLink := fmt.Sprintf(
		`=HYPERLINK("%v", "%v")`,
		"https://"+proxyAddr+"/web/requests/"+ar.GetName(),
		"View Access Request",
	)

	return &sheets.RowData{
		Values: []*sheets.CellData{
			&sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{
					StringValue: stringPtr(ar.GetName()),
				},
			},
			&sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{
					StringValue: stringPtr(ar.GetCreationTime().String()),
				},
			},
			&sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{
					StringValue: stringPtr(ar.GetUser()),
				},
			},
			&sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{
					StringValue: stringPtr(strings.Join(ar.GetRoles(), ",")),
				},
			},
			&sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{
					StringValue: &requestState,
				},
			},
			&sheets.CellData{
				UserEnteredValue: &sheets.ExtendedValue{
					FormulaValue: &viewLink,
				},
			},
		},
	}
}

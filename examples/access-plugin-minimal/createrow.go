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

	"github.com/gravitational/trace"
	sheets "google.golang.org/api/sheets/v4"

	"github.com/gravitational/teleport/api/types"
)

func (g *googleSheetsClient) createRow(ar types.AccessRequest) error {
	row := g.makeRowData(ar)

	req := sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				AppendCells: &sheets.AppendCellsRequest{

					Fields: "*",
					Rows: []*sheets.RowData{
						row,
					},
				},
			},
		},
	}

	resp, err := g.sheetsClient.BatchUpdate(spreadSheetID, &req).Do()
	if err != nil {
		return trace.Wrap(err)
	}

	if resp.HTTPStatusCode == 201 || resp.HTTPStatusCode == 200 {
		fmt.Println("Successfully created a row")
	} else {
		fmt.Printf(
			"Unexpected response code creating a row: %v\n",
			resp.HTTPStatusCode,
		)
	}

	return nil

}

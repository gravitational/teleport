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
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func (g *googleSheetsClient) updateSpreadsheet(ar types.AccessRequest) error {
	s, err := g.sheetsClient.Get(spreadSheetID).IncludeGridData(true).Do()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(s.Sheets) != 1 {
		return trace.Wrap(
			errors.New("the spreadsheet must have a single sheet"),
		)
	}

	for _, d := range s.Sheets[0].Data {
		for i, r := range d.RowData {
			if r.Values[0] != nil &&
				r.Values[0].UserEnteredValue != nil &&
				r.Values[0].UserEnteredValue.StringValue != nil &&
				*r.Values[0].UserEnteredValue.StringValue == ar.GetName() {
				if err := g.updateRow(ar, int64(i)); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
	return nil
}

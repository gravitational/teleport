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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport-plugins/lib"
	"github.com/gravitational/teleport-plugins/lib/watcherjob"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"google.golang.org/api/option"
	sheets "google.golang.org/api/sheets/v4"
	"google.golang.org/grpc"
)

const (
	proxyAddr     string = ""
	initTimeout          = time.Duration(30) * time.Second
	spreadSheetID string = ""
)

var requestStates = map[types.RequestState]string{
	types.RequestState_APPROVED: "APPROVED",
	types.RequestState_DENIED:   "DENIED",
	types.RequestState_PENDING:  "PENDING",
	types.RequestState_NONE:     "NONE",
}

type googleSheetsPlugin struct {
	sheetsClient   *sheets.SpreadsheetsService
	teleportClient *client.Client
}

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

func (g *googleSheetsPlugin) createRow(ar types.AccessRequest) error {
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

func (g *googleSheetsPlugin) updateRow(ar types.AccessRequest, rowNum int64) error {
	row := g.makeRowData(ar)

	req := sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				UpdateCells: &sheets.UpdateCellsRequest{

					Fields: "*",
					Start: &sheets.GridCoordinate{
						RowIndex: rowNum,
					},
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
		fmt.Println("Successfully updated a row")
	} else {
		fmt.Printf(
			"Unexpected response code updating a row: %v\n",
			resp.HTTPStatusCode,
		)
	}

	return nil

}

func (g *googleSheetsPlugin) updateSpreadsheet(ar types.AccessRequest) error {
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
				fmt.Println("Updated a spreadsheet row.")
			}
		}
	}
	return nil
}

func (g *googleSheetsPlugin) handleEvent(ctx context.Context, event types.Event) error {

	if event.Resource == nil {
		return nil
	}

	r := event.Resource.(types.AccessRequest)

	if r.GetState() == types.RequestState_PENDING {
		return g.createRow(r)
	}

	return g.updateSpreadsheet(r)
}

func (g *googleSheetsPlugin) run() error {
	ctx := context.Background()
	proc := lib.NewProcess(ctx)
	watcherJob := watcherjob.NewJob(
		g.teleportClient,
		watcherjob.Config{
			Watch: types.Watch{Kinds: []types.WatchKind{types.WatchKind{Kind: types.KindAccessRequest}}},
		},
		g.handleEvent,
	)

	proc.SpawnCriticalJob(watcherJob)

	fmt.Println("Started the watcher job")

	<-watcherJob.Done()

	fmt.Println("The watcher job is finished")

	return nil
}

func main() {
	ctx := context.Background()
	svc, err := sheets.NewService(ctx, option.WithCredentialsFile("credentials.json"))
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	creds := client.LoadIdentityFile("auth.pem")

	teleport, err := client.New(ctx, client.Config{
		Addrs:       []string{proxyAddr},
		Credentials: []client.Credentials{creds},
		DialOpts: []grpc.DialOption{
			grpc.WithReturnConnectionError(),
		},
	})
	if err != nil {
		panic(err)
	}

	gs := googleSheetsPlugin{
		sheetsClient:   sheets.NewSpreadsheetsService(svc),
		teleportClient: teleport,
	}

	if err := gs.run(); err != nil {
		panic(err)
	}
}

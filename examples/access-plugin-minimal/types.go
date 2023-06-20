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
	"time"

	sheets "google.golang.org/api/sheets/v4"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
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

type AccessRequestPlugin struct {
	TeleportClient *client.Client
	EventHandler   interface {
		HandleEvent(ctx context.Context, event types.Event) error
	}
}

type googleSheetsClient struct {
	sheetsClient *sheets.SpreadsheetsService
}

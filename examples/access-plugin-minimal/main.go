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

	"google.golang.org/api/option"
	sheets "google.golang.org/api/sheets/v4"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client"
)

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

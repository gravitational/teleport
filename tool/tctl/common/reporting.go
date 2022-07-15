/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/lib/auth"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func displayLicenseWarnings(ctx context.Context, client auth.ClientI) error {
	clt, ok := client.(*auth.Client)
	if !ok {
		trace.BadParameter("expected *auth.Client, got: %T", clt)
	}
	var header metadata.MD
	_, err := clt.WithCallOptions(grpc.Header(&header)).Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, warning := range header.Get("license-warnings") {
		fmt.Println(warning)
	}
	return nil
}

// Copyright 2021 Gravitational, Inc
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

package teleterm_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/teleterm"

	"github.com/stretchr/testify/require"
)

func TestStart(t *testing.T) {
	homeDir := t.TempDir()
	cfg := teleterm.Config{
		Addr:    fmt.Sprintf("unix://%v/teleterm.sock", homeDir),
		HomeDir: fmt.Sprintf("%v/", homeDir),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wait := make(chan error)
	go func() {
		err := teleterm.Start(ctx, cfg)
		wait <- err
	}()

	defer func() {
		cancel() // Stop the server.
		require.NoError(t, <-wait)
	}()

}

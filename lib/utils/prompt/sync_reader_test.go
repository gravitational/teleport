// Copyright 2022 Gravitational, Inc
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

package prompt_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/stretchr/testify/require"
)

func TestSyncReader_ReadContext(t *testing.T) {
	ctx := context.Background()
	data := []byte("who let the llamas out?")

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	tests := []struct {
		name     string
		data     []byte
		ctx      context.Context
		wantData []byte
		wantErr  error
	}{
		{
			name:     "ok",
			data:     data,
			ctx:      ctx,
			wantData: data,
		},
		{
			name:    "ctx cancelled",
			data:    data,
			ctx:     cancelledCtx,
			wantErr: cancelledCtx.Err(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &prompt.SyncReader{
				Reader: bytes.NewReader(test.data),
			}
			got, err := r.ReadContext(test.ctx)
			require.Equal(t, test.wantErr, err, "ReadContext error mismatch")
			if err != nil {
				return
			}
			require.Equal(t, test.wantData, got, "ReadContext data mismatch")
		})
	}
}

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

package types_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestResource153ToLegacy(t *testing.T) {
	// bot is an example of an RFD 153 "compliant" resource.
	// Any other resource type would do.
	bot := &machineidv1.Bot{
		Kind:     "bot",
		SubKind:  "robot",
		Version:  "",
		Metadata: &headerv1.Metadata{Name: "Bernard"},
		Spec: &machineidv1.BotSpec{
			Roles: []string{"robot", "human"},
		},
	}

	legacyResource := types.Resource153ToLegacy(bot)

	// Unwrap gives the underlying resource back.
	t.Run("unwrap", func(t *testing.T) {
		unwrapped := legacyResource.(interface{ Unwrap() types.Resource153 }).Unwrap()
		if diff := cmp.Diff(bot, unwrapped, protocmp.Transform()); diff != "" {
			t.Errorf("Unwrap mismatch (-want +got)\n%s", diff)
		}
	})

	// Marshaling as JSON marshals the underlying resource.
	t.Run("marshal", func(t *testing.T) {
		jsonBytes, err := json.Marshal(legacyResource)
		require.NoError(t, err, "Marshal")

		bot2 := &machineidv1.Bot{}
		require.NoError(t, json.Unmarshal(jsonBytes, bot2), "Unmarshal")
		if diff := cmp.Diff(bot, bot2, protocmp.Transform()); diff != "" {
			t.Errorf("Marshal/Unmarshal mismatch (-want +got)\n%s", diff)
		}
	})
}

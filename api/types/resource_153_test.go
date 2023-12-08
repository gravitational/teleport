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
	"github.com/gravitational/teleport/api/types"
)

func TestResource153ToLegacy(t *testing.T) {
	// fake153Resource is used here because, at the time of backport, there were
	// no RFD 153 resources in this branch.
	bot := &fake153Resource{
		Kind:     "bot",
		SubKind:  "robot",
		Metadata: &headerv1.Metadata{Name: "Bernard"},
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

		bot2 := &fake153Resource{}
		require.NoError(t, json.Unmarshal(jsonBytes, bot2), "Unmarshal")
		if diff := cmp.Diff(bot, bot2, protocmp.Transform()); diff != "" {
			t.Errorf("Marshal/Unmarshal mismatch (-want +got)\n%s", diff)
		}
	})
}

type fake153Resource struct {
	Kind     string
	SubKind  string
	Version  string
	Metadata *headerv1.Metadata
}

func (r *fake153Resource) GetKind() string {
	return r.Kind
}

func (r *fake153Resource) GetMetadata() *headerv1.Metadata {
	return r.Metadata
}

func (r *fake153Resource) GetSubKind() string {
	return r.SubKind
}

func (r *fake153Resource) GetVersion() string {
	return r.Version
}

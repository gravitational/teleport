// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"testing"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/asciitable"
)

func makeInferenceSecret(name string) *summarizerv1.InferenceSecret {
	return summarizer.NewInferenceSecret(name, &summarizerv1.InferenceSecretSpec{
		Value: "some-value",
	})
}

func TestInferenceSecretCollection_writeText(t *testing.T) {
	secrets := []*summarizerv1.InferenceSecret{
		makeInferenceSecret("secret_1"),
		makeInferenceSecret("secret_2"),
		makeInferenceSecret("secret_3"),
	}

	table := asciitable.MakeTable(
		[]string{"Name", "Description"},
		[]string{"secret_1", ""},
		[]string{"secret_2", ""},
		[]string{"secret_3", ""},
	)

	formatted := table.AsBuffer().String()

	collectionFormatTest(t, inferenceSecretCollection(secrets), formatted, formatted)
}

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

func makeInferenceModel(name, modelSuffix string) *summarizerv1.InferenceModel {
	return summarizer.NewInferenceModel(name, &summarizerv1.InferenceModelSpec{
		Provider: &summarizerv1.InferenceModelSpec_Openai{
			Openai: &summarizerv1.OpenAIProvider{
				OpenaiModelId: "gpt-4o" + modelSuffix,
			},
		},
	})
}

func TestInferenceModelCollection_writeText(t *testing.T) {
	models := []*summarizerv1.InferenceModel{
		makeInferenceModel("model_1", "-1"),
		makeInferenceModel("model_2", "-2"),
		makeInferenceModel("model_3", "-3"),
	}

	table := asciitable.MakeTable(
		[]string{"Name", "Description", "Provider", "Provider Model ID"},
		[]string{"model_1", "", "OpenAI", "gpt-4o-1"},
		[]string{"model_2", "", "OpenAI", "gpt-4o-2"},
		[]string{"model_3", "", "OpenAI", "gpt-4o-3"},
	)

	formatted := table.AsBuffer().String()

	collectionFormatTest(t, inferenceModelCollection(models), formatted, formatted)
}

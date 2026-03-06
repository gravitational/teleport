// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

func makeInferencePolicy(name, modelSuffix string) *summarizerv1.InferencePolicy {
	return summarizer.NewInferencePolicy(name, &summarizerv1.InferencePolicySpec{
		Kinds:  []string{"ssh"},
		Filter: `resource.metadata.labels["env"] == "prod"`,
		Model:  "some-model" + modelSuffix,
	})
}

func TestInferencePolicyCollection_writeText(t *testing.T) {
	policies := []*summarizerv1.InferencePolicy{
		makeInferencePolicy("policy_1", "-1"),
		makeInferencePolicy("policy_2", "-2"),
		makeInferencePolicy("policy_3", "-3"),
	}

	headers := []string{"Name", "Description", "Kinds", "Filter", "Model"}
	rows := [][]string{
		{"policy_1", "", "ssh", `resource.metadata.labels["env"] == "prod"`, "some-model-1"},
		{"policy_2", "", "ssh", `resource.metadata.labels["env"] == "prod"`, "some-model-2"},
		{"policy_3", "", "ssh", `resource.metadata.labels["env"] == "prod"`, "some-model-3"},
	}

	table := asciitable.MakeTableWithTruncatedColumn(headers, rows, "Description")
	formatted := table.AsBuffer().String()

	verboseTable := asciitable.MakeTable(headers, rows...)
	verboseFormatted := verboseTable.AsBuffer().String()

	collectionFormatTest(t, inferencePolicyCollection(policies), verboseFormatted, formatted)
}

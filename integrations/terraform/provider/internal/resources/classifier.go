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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/summarizer/v1"
)

// NewClassifierDataSourceType returns the classifier data source type.
func NewClassifierDataSourceType() tfdriver.DataSourceType[summarizerv1.Classifier, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[summarizerv1.Classifier, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[summarizerv1.Classifier, tfdriver.NameIdentifier] {
			return teleport.NewClassifierClient(clientFromProvider(p))
		},
		Kind: types.KindClassifier,
		Codec: tfdriver.DataSourceCodecFuncs[summarizerv1.Classifier]{
			SchemaFunc:  schemav1.GenSchemaClassifier,
			ToStateFunc: schemav1.CopyClassifierToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewClassifierResourceType returns the classifier resource type.
func NewClassifierResourceType() tfdriver.ResourceType[summarizerv1.Classifier, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[summarizerv1.Classifier, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[summarizerv1.Classifier, tfdriver.NameIdentifier] {
			return teleport.NewClassifierClient(clientFromProvider(p))
		},
		Kind: types.KindClassifier,
		Codec: tfdriver.ResourceCodecFuncs[summarizerv1.Classifier]{
			SchemaFunc:   schemav1.GenSchemaClassifier,
			ToStateFunc:  schemav1.CopyClassifierToTerraform,
			FromPlanFunc: schemav1.CopyClassifierFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[summarizerv1.Classifier](types.KindClassifier),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(classifier *summarizerv1.Classifier) string {
			return classifier.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *summarizerv1.Classifier) string {
			return st.GetMetadata().GetRevision()
		},
	}
}

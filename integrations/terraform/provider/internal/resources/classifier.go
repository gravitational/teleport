// Copyright 2026 Gravitational, Inc.
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

package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/summarizer/v1"
)

// NewClassifierDataSourceType returns the classifier data source type.
func NewClassifierDataSourceType() tfdriver.DataSourceType[summarizerv1.Classifier, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[summarizerv1.Classifier, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[summarizerv1.Classifier, tfdriver.NameIdentifier] {
			return teleport.NewClassifierClient(clientFromProvider(p))
		},
		Kind: types.KindClassifier,
		Name: types.KindClassifier,
		Codec: tfdriver.DataSourceCodecFuncs[summarizerv1.Classifier]{
			SchemaFunc:  schemav1.GenSchemaClassifierDataSource,
			ToStateFunc: schemav1.CopyClassifierToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewClassifierResourceType returns the classifier resource type.
func NewClassifierResourceType() tfdriver.ResourceType[summarizerv1.Classifier, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[summarizerv1.Classifier, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[summarizerv1.Classifier, tfdriver.NameIdentifier] {
			return teleport.NewClassifierClient(clientFromProvider(p))
		},
		Kind: types.KindClassifier,
		Name: types.KindClassifier,
		Codec: tfdriver.ResourceCodecFuncs[summarizerv1.Classifier]{
			SchemaFunc:   schemav1.GenSchemaClassifierResource,
			ToStateFunc:  schemav1.CopyClassifierToTerraform,
			FromPlanFunc: schemav1.CopyClassifierFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyClassifierFromTerraform,
				schemav1.CopyClassifierToTerraformPreserveUnknown,
			),
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

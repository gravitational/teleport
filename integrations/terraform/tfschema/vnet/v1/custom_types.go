// Copyright 2025 Gravitational, Inc.
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

package v1

import (
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
	"github.com/gravitational/teleport/integrations/terraform/tfschema/resource153"
)

type DurationType = tfschema.DurationType

type DurationValue = tfschema.DurationValue

var (
	GenSchemaTimestampDataSource = resource153.GenSchemaTimestampDataSource
	GenSchemaTimestampResource   = resource153.GenSchemaTimestampResource
	CopyToTimestamp              = resource153.CopyToTimestamp
	CopyFromTimestamp            = resource153.CopyFromTimestamp

	GenSchemaLabelsDataSource = resource153.GenSchemaLabelsDataSource
	GenSchemaLabelsResource   = resource153.GenSchemaLabelsResource
	CopyFromLabels            = resource153.CopyFromLabels
	CopyToLabels              = resource153.CopyToLabels
)

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package awsactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// DocumentCreator can create an AWS SSM document.
type DocumentCreator interface {
	// CreateDocument creates an AWS SSM document.
	CreateDocument(ctx context.Context, params *ssm.CreateDocumentInput, optFns ...func(*ssm.Options)) (*ssm.CreateDocumentOutput, error)
}

// CreateDocument wraps a [DocumentCreator] in a [provisioning.Action] that
// creates an SSM document when invoked.
func CreateDocument(
	clt DocumentCreator,
	name string,
	content string,
	docType ssmtypes.DocumentType,
	docFormat ssmtypes.DocumentFormat,
	tags tags.AWSTags,
) (*provisioning.Action, error) {
	input := &ssm.CreateDocumentInput{
		Name:           aws.String(name),
		DocumentType:   docType,
		DocumentFormat: docFormat,
		Content:        aws.String(content),
		Tags:           tags.ToSSMTags(),
	}
	type createDocumentInput struct {
		// PolicyDocument shadows the input's field of the same name
		// to marshal the doc content as unescaped JSON or text.
		Content any
		*ssm.CreateDocumentInput
	}
	unmarshaledContent, err := unmarshalDocumentContent(content, docFormat)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	details, err := formatDetails(createDocumentInput{
		Content:             unmarshaledContent,
		CreateDocumentInput: input,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "CreateDocument",
		Summary: fmt.Sprintf("Create an AWS Systems Manager (SSM) %s document %q", docType, name),
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			_, err = clt.CreateDocument(ctx, input)
			if err != nil {
				var docAlreadyExistsError *ssmtypes.DocumentAlreadyExists
				if errors.As(err, &docAlreadyExistsError) {
					slog.InfoContext(ctx, "SSM document already exists", "name", name)
					return nil
				}

				return trace.Wrap(err)
			}

			slog.InfoContext(ctx, "SSM document created", "name", name)
			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}

func unmarshalDocumentContent(content string, docFormat ssmtypes.DocumentFormat) (any, error) {
	var structuredOutput map[string]any
	switch docFormat {
	case ssmtypes.DocumentFormatJson:
		json.Unmarshal([]byte(content), &structuredOutput)
	case ssmtypes.DocumentFormatYaml:
		yaml.Unmarshal([]byte(content), &structuredOutput)
	case ssmtypes.DocumentFormatText:
		return content, nil
	default:
		return nil, trace.BadParameter("unknown document format %q", docFormat)
	}

	return structuredOutput, nil
}

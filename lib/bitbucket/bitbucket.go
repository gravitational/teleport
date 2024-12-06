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

package bitbucket

import (
	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
)

// IDTokenClaims
// See the following for the structure:
// https://support.atlassian.com/bitbucket-cloud/docs/integrate-pipelines-with-resource-servers-using-oidc/
type IDTokenClaims struct {
	// Sub provides some information about the Bitbucket Pipelines run that
	// generated this token. Format: {RepositoryUUID}:{StepUUID}
	Sub string `json:"sub"`

	// StepUUID is the UUID of the pipeline step for which this token was
	// issued. Bitbucket UUIDs must begin and end with braces, e.g. '{...}'
	StepUUID string `json:"stepUuid"`

	// RepositoryUUID is the UUID of the repository for which this token was
	// issued. Bitbucket UUIDs must begin and end with braces, e.g. '{...}'.
	// This value may be found in the Pipelines -> OpenID Connect section of the
	// repository settings.
	RepositoryUUID string `json:"repositoryUuid"`

	// PipelineUUID is the UUID of the pipeline for which this token was issued.
	// Bitbucket UUIDs must begin and end with braces, e.g. '{...}'
	PipelineUUID string `json:"pipelineUuid"`

	// WorkspaceUUID is the UUID of the workspace for which this token was
	// issued. Bitbucket UUIDs must begin and end with braces, e.g. '{...}'.
	// This value may be found in the Pipelines -> OpenID Connect section of the
	// repository settings.
	WorkspaceUUID string `json:"workspaceUuid"`

	// DeploymentEnvironmentUUID is the name of the deployment environment for
	// which this pipeline was executed. Bitbucket UUIDs must begin and end with
	// braces, e.g. '{...}'.
	DeploymentEnvironmentUUID string `json:"deploymentEnvironmentUuid"`

	// BranchName is the name of the branch on which this pipeline executed.
	BranchName string `json:"branchName"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *IDTokenClaims) JoinAuditAttributes() (map[string]any, error) {
	res := map[string]any{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &res,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := d.Decode(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}

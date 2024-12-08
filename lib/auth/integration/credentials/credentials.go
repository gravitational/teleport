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

package credentials

import (
	"context"
	"maps"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Package credentials defines constants and provides helper functions for
// integration credentials.

const (
	// LabelStaticCredentialsIntegration is the label used to store the
	// UUID ref in the static credentials.
	LabelStaticCredentialsIntegration = types.TeleportInternalLabelPrefix + types.KindIntegration
	// LabelStaticCredentialsPurpose is the label used to store the purpose of
	// the static credentials.
	LabelStaticCredentialsPurpose = "purpose"

	// PurposeGitHubSSHCA is the label value that indicates the static
	// credentials contains the GitHub SSH CA.
	PurposeGitHubSSHCA = "github-sshca"
	// PurposeGitHubOAuth is the label value that indicates the static
	// credentials contains the GitHub OAuth ID and secret.
	PurposeGitHubOAuth = "github-oauth"
)

// NewRef creates a new PluginStaticCredentialsRef that is saved along with the
// integration resource in the backend. The actual credentials are saved as
// PlugStaticCredentials and can only be retrieved by the ref.
func NewRef() *types.PluginStaticCredentialsRef {
	return NewRefWithUUID(uuid.NewString())
}

// NewRefWithUUID creates a PluginStaticCredentialsRef with provided UUID.
func NewRefWithUUID(uuid string) *types.PluginStaticCredentialsRef {
	return &types.PluginStaticCredentialsRef{
		Labels: map[string]string{
			LabelStaticCredentialsIntegration: uuid,
		},
	}
}

// CopyRefLabels copies the labels from the Ref to the actual credentials so the
// credentials can be retrieved using the same labels.
func CopyRefLabels(cred types.PluginStaticCredentials, ref *types.PluginStaticCredentialsRef) {
	labels := cred.GetStaticLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, ref.Labels)

	cred.SetStaticLabels(labels)
}

// ByLabelsGetter defines an interface to retrieve credentials by labels.
type ByLabelsGetter interface {
	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}

// GetByPurpose retrieves a credentials based on the provided purpose.
func GetByPurpose(ctx context.Context, ref *types.PluginStaticCredentialsRef, purpose string, getter ByLabelsGetter) (types.PluginStaticCredentials, error) {
	if ref == nil {
		return nil, trace.BadParameter("missing credentials ref")
	}
	labels := ref.Labels
	if len(labels) == 0 {
		return nil, trace.BadParameter("missing labels from credentials ref")
	}
	labels[LabelStaticCredentialsPurpose] = purpose

	creds, err := getter.GetPluginStaticCredentialsByLabels(ctx, labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch len(creds) {
	case 0:
		return nil, trace.NotFound("%v credentials not found", purpose)
	case 1:
		return creds[0], nil
	default:
		return nil, trace.CompareFailed("expecting one plugin static credentials but got %v", len(creds))
	}
}

// IntegrationGetter defines an interface to retrieve an integration by name.
type IntegrationGetter interface {
	// GetIntegration returns the specified integration resources.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// GetIntegrationRef is a helper to get the PluginStaticCredentialsRef from the
// integration.
func GetIntegrationRef(ctx context.Context, integration string, igGetter IntegrationGetter) (*types.PluginStaticCredentialsRef, error) {
	ig, err := igGetter.GetIntegration(ctx, integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cred := ig.GetCredentials()
	if cred == nil {
		return nil, trace.BadParameter("no credentials found for %q", integration)
	}

	ref := cred.GetStaticCredentialsRef()
	if ref == nil {
		return nil, trace.BadParameter("no credentials ref found for %q", integration)
	}
	return ref, nil
}

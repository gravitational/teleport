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

package gcp

import (
	"context"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func makeMetadataGetter(values map[string]string) MetadataGetter {
	return func(ctx context.Context, path string) (string, error) {
		value, ok := values[path]
		if ok {
			return value, nil
		}
		return "", trace.NotFound("no value for %v", path)
	}
}

type mockInstanceGetter struct {
	InstanceGetter
	instance    *Instance
	instanceErr error
	tags        map[string]string
	tagsErr     error
}

func (m *mockInstanceGetter) GetInstance(ctx context.Context, req *InstanceRequest) (*Instance, error) {
	return m.instance, m.instanceErr
}

func (m *mockInstanceGetter) GetInstanceTags(ctx context.Context, req *InstanceRequest) (map[string]string, error) {
	return m.tags, m.tagsErr
}

func TestIsInstanceMetadataAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		getMetadata MetadataGetter
		assert      require.BoolAssertionFunc
	}{
		{
			name: "not available",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "", trace.NotFound("")
			},
			assert: require.False,
		},
		{
			name: "not on gcp",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "non-numeric id", nil
			},
			assert: require.False,
		},
		{
			name: "zero ID",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "0", nil
			},
			assert: require.False,
		},
		{
			name: "on mocked gcp",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "12345678", nil
			},
			assert: require.True,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &InstanceMetadataClient{
				getMetadata: tc.getMetadata,
			}
			tc.assert(t, client.IsAvailable(context.Background()))
		})
	}

	t.Run("on real gcp", func(t *testing.T) {
		if os.Getenv("TELEPORT_TEST_GCP") == "" {
			t.Skip("not on gcp")
		}
		client, err := NewInstanceMetadataClient(nil)
		require.NoError(t, err)
		require.True(t, client.IsAvailable(context.Background()))
	})
}

func TestGetTags(t *testing.T) {
	t.Parallel()

	defaultMetadataGetter := makeMetadataGetter(map[string]string{
		"project/project-id": "myproject",
		"instance/zone":      "myzone",
		"instance/name":      "myname",
		"instance/id":        "12345678",
	})
	defaultInstance := &Instance{
		ProjectID: "myproject",
		Zone:      "myzone",
		Name:      "myname",
		Labels: map[string]string{
			"foo": "bar",
		},
	}

	tests := []struct {
		name            string
		getMetadata     MetadataGetter
		instancesClient *mockInstanceGetter
		assertErr       require.ErrorAssertionFunc
		expectedTags    map[string]string
	}{
		{
			name:        "ok",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instance: defaultInstance,
				tags: map[string]string{
					"baz": "quux",
				},
			},
			assertErr: require.NoError,
			expectedTags: map[string]string{
				"label/foo": "bar",
				"tag/baz":   "quux",
			},
		},
		{
			name:        "not on gcp",
			getMetadata: makeMetadataGetter(nil),
			instancesClient: &mockInstanceGetter{
				instance: defaultInstance,
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err), i...)
			},
		},
		{
			name:        "instance not found",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instanceErr: trace.NotFound(""),
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err), i...)
			},
		},
		{
			name:        "denied access to instance",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instanceErr: trace.AccessDenied(""),
				tags: map[string]string{
					"baz": "quux",
				},
			},
			assertErr: require.NoError,
			expectedTags: map[string]string{
				"tag/baz": "quux",
			},
		},
		{
			name:        "denied access to tags",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instance: defaultInstance,
				tagsErr:  trace.AccessDenied(""),
			},
			assertErr: require.NoError,
			expectedTags: map[string]string{
				"label/foo": "bar",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &InstanceMetadataClient{
				getMetadata:    tc.getMetadata,
				instanceGetter: tc.instancesClient,
			}
			tags, err := client.GetTags(context.Background())
			tc.assertErr(t, err)
			require.Equal(t, tc.expectedTags, tags)
		})
	}
}

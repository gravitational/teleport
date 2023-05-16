// Copyright 2023 Gravitational, Inc
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

package plugindata

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

const (
	resourceKind = "test"
)

type mockData struct {
	Foo string
	Bar string
}

func mockEncode(source mockData) (map[string]string, error) {
	result := make(map[string]string)

	result["foo"] = source.Foo
	result["bar"] = source.Bar

	return result, nil
}

func mockDecode(source map[string]string) (mockData, error) {
	result := mockData{}

	result.Foo = source["foo"]
	result.Bar = source["bar"]

	return result, nil
}

func mockDecodeFail(source map[string]string) (mockData, error) {
	return mockData{}, trace.BadParameter("Failed to decode data")
}

type mockClient struct {
	oldDataCursor      int
	oldData            []map[string]string
	updateResult       []error
	updateResultCursor int
}

func (c *mockClient) GetPluginData(_ context.Context, f types.PluginDataFilter) ([]types.PluginData, error) {
	i, err := types.NewPluginData(f.Resource, resourceKind)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	d, ok := i.(*types.PluginDataV3)
	if !ok {
		return nil, trace.Errorf("Failed to convert %T to types.PluginDataV3", i)
	}

	var data map[string]string
	if c.oldDataCursor < len(c.oldData) {
		data = c.oldData[c.oldDataCursor]
	}
	c.oldDataCursor++

	d.Spec.Entries = map[string]*types.PluginDataEntry{
		resourceKind: {Data: data},
	}

	return []types.PluginData{d}, nil
}

func (c *mockClient) UpdatePluginData(context.Context, types.PluginDataUpdateParams) error {
	if c.updateResultCursor+1 > len(c.updateResult) {
		return nil
	}
	err := c.updateResult[c.updateResultCursor]
	c.updateResultCursor++
	return err
}

func TestModifyFailed(t *testing.T) {
	c := &mockClient{
		oldData: []map[string]string{{"foo": "value"}},
	}
	cas := NewCAS(c, resourceKind, types.KindAccessRequest, mockEncode, mockDecode)

	r, err := cas.Update(context.Background(), "foo", func(data mockData) (mockData, error) {
		return mockData{}, trace.Errorf("fail")
	})

	require.Error(t, err, "fail")
	require.Equal(t, r, mockData{})
}

// We test cas is retrying modityT properly if modifyT returns a CompareFailedError during the first iteration.
func TestModifyCompareFailed(t *testing.T) {
	c := &mockClient{
		oldData: []map[string]string{
			{"foo": "0"},
			{"foo": "1"},
		},
	}
	cas := NewCAS(c, resourceKind, types.KindAccessRequest, mockEncode, mockDecode)

	r, err := cas.Update(context.Background(), "foo", func(data mockData) (mockData, error) {
		// If this is the first time we're called we fail
		if data.Foo == "0" {
			return mockData{}, &trace.CompareFailedError{Message: "does not exist yet"}
		}
		data.Bar = "other value"
		return data, nil
	})

	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, r.Bar, "other value")
}

func TestModifySuccess(t *testing.T) {
	c := &mockClient{
		oldData: []map[string]string{{"foo": "value"}},
	}
	cas := NewCAS(c, resourceKind, types.KindAccessRequest, mockEncode, mockDecode)

	r, err := cas.Update(context.Background(), "foo", func(i mockData) (mockData, error) {
		i.Foo = "other value"
		return i, nil
	})

	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, r.Foo, "other value")
}

func TestBackoff(t *testing.T) {
	c := &mockClient{
		oldData:      []map[string]string{{"foo": "value"}, {"foo": "value"}},
		updateResult: []error{trace.CompareFailed("fail"), nil},
	}
	cas := NewCAS(c, resourceKind, types.KindAccessRequest, mockEncode, mockDecode)

	r, err := cas.Update(context.Background(), "foo", func(_ mockData) (mockData, error) {
		return mockData{Foo: "yes"}, nil
	})

	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, r.Foo, "yes")
}

func TestWrongData(t *testing.T) {
	c := &mockClient{
		oldData: []map[string]string{{"foo": "value"}},
	}
	cas := NewCAS(c, resourceKind, types.KindAccessRequest, mockEncode, mockDecodeFail)

	_, err := cas.Update(context.Background(), "foo", func(i mockData) (mockData, error) {
		i.Foo = "other value"
		return i, nil
	})

	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("Failed to decode data"))
}

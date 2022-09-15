//go:build !windows
// +build !windows

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/stretchr/testify/require"
)

// simple test to verify that compatibility layer is reading and dumping the keys.
func Test_copyLocalStorageIntoKubernetes(t *testing.T) {
	litebk, err := lite.NewWithConfig(context.TODO(), lite.Config{
		Path:      t.TempDir(),
		EventsOff: true,
		Sync:      lite.SyncFull,
	})
	require.NoError(t, err)
	defer litebk.Close()
	stateKeys := []backend.Item{
		{
			Key:   backend.Key(idsPrefix, "kube", "current"),
			Value: []byte("test1"),
		},
		{
			Key:   backend.Key(idsPrefix, "app", "current"),
			Value: []byte("test2"),
		},
		{
			Key:   backend.Key(idsPrefix, "kube", "replacement"),
			Value: []byte("test3"),
		},
		{
			Key:   backend.Key(statesPrefix, "kube", "replacement"),
			Value: []byte("test4"),
		},
	}

	err = litebk.PutRange(context.TODO(), stateKeys)
	require.NoError(t, err)

	// write another key that is neither an identity neither a state
	_, err = litebk.Put(
		context.TODO(),
		backend.Item{
			Key:   backend.Key("nonIDOrState", "kube", "replacement"),
			Value: []byte("test5"),
		},
	)
	require.NoError(t, err)

	// simple mock to catch the keys
	kubeMock := &k8sStorageMock{}
	err = copyLocalStorageIntoKubernetes(context.TODO(), kubeMock, litebk)
	require.NoError(t, err)

	// SQLite returns keys in alphabetical order
	sort.Slice(stateKeys, func(i, j int) bool {
		return string(stateKeys[i].Key) < string(stateKeys[j].Key)
	})
	require.Equal(t, stateKeys, kubeMock.items)
}

type k8sStorageMock struct {
	items []backend.Item
}

func (c *k8sStorageMock) PutRange(_ context.Context, items []backend.Item) error {
	c.items = items
	return nil
}

func (c *k8sStorageMock) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *k8sStorageMock) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *k8sStorageMock) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	return nil, fmt.Errorf("not implemented")
}

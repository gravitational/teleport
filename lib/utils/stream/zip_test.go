/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stream_test

import (
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/internalutils/stream"
	streamutils "github.com/gravitational/teleport/lib/utils/stream"
)

func Test_zipStreams_Process(t *testing.T) {
	type testCase[T any, V any] struct {
		name     string
		validate func(t *testing.T) (*streamutils.ZipStreams[T, V], func())
		wantErr  bool
	}
	tests := []testCase[string, string]{
		{
			name: "empty",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				counter := 0
				equalCounter := 0
				return streamutils.NewZipStreams[string, string](
						stream.Empty[string](),
						stream.Empty[string](),
						func(s1 string) error {
							counter++
							return nil
						},
						func(leader string, follower string) error {
							equalCounter++
							return nil
						},
						strings.Compare,
					), func() {
						require.Equal(t, 0, counter)
						require.Equal(t, 0, equalCounter)
					}
			},
			wantErr: false,
		},
		{
			name: "one",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				counter := 0
				equalCounter := 0
				return streamutils.NewZipStreams[string, string](
						stream.Slice([]string{"foo"}),
						stream.Empty[string](),
						func(s1 string) error {
							counter++
							return nil
						},
						func(leader string, follower string) error {
							equalCounter++
							return nil
						},
						strings.Compare,
					), func() {
						require.Equal(t, 1, counter)
						require.Equal(t, 0, equalCounter)
					}
			},
			wantErr: false,
		},
		{
			name: "no leaders",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				counter := 0
				equalCounter := 0
				return streamutils.NewZipStreams[string, string](
						stream.Empty[string](),
						stream.Slice([]string{"foo"}),
						func(s1 string) error {
							counter++
							return nil
						},
						func(leader string, follower string) error {
							equalCounter++
							return nil
						},
						strings.Compare,
					), func() {
						require.Equal(t, 0, counter)
						require.Equal(t, 0, equalCounter)
					}
			},
			wantErr: false,
		},
		{
			name: "already in sync",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				counter := 0
				equalCounter := 0
				return streamutils.NewZipStreams[string, string](
						stream.Slice([]string{"foo"}),
						stream.Slice([]string{"foo"}),
						func(s1 string) error {
							counter++
							return nil
						},
						func(leader string, follower string) error {
							equalCounter++
							return nil
						},
						strings.Compare,
					), func() {
						require.Equal(t, 0, counter)
						require.Equal(t, 1, equalCounter)
					}
			},
			wantErr: false,
		},
		{
			name: "additional leader",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				counter := 0
				equalCounter := 0
				calledWith := make([]string, 0)
				return streamutils.NewZipStreams[string, string](
						stream.Slice([]string{"bar", "foo"}),
						stream.Slice([]string{"foo"}),
						func(s1 string) error {
							counter++
							calledWith = append(calledWith, s1)
							return nil
						},
						func(leader string, follower string) error {
							// should be called with "foo" and "foo"
							require.Equal(t, "foo", leader)
							require.Equal(t, "foo", follower)
							equalCounter++
							return nil
						},
						strings.Compare,
					), func() {
						require.Equal(t, 1, counter)
						require.Equal(t, []string{"bar"}, calledWith)
						require.Equal(t, 1, equalCounter)
					}
			},
			wantErr: false,
		},
		{
			name: "additional follower - no calls",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				counter := 0
				equalCounter := 0
				return streamutils.NewZipStreams[string, string](
						stream.Slice([]string{"foo"}),
						stream.Slice([]string{"bar", "foo"}),
						func(s1 string) error {
							counter++
							return nil
						},
						func(leader string, follower string) error {
							require.Equal(t, "foo", leader)
							require.Equal(t, "foo", follower)
							equalCounter++
							return nil
						},
						strings.Compare,
					), func() {
						require.Equal(t, 0, counter)
						require.Equal(t, 1, equalCounter)
					}
			},
			wantErr: false,
		},
		{
			name: "mix",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				counter := 0
				equalCount := 0
				calledWith := make([]string, 0)
				sameCalledWith := make([]string, 0)
				return streamutils.NewZipStreams[string, string](
						stream.Slice([]string{"1", "2", "4", "5", "8"}),
						stream.Slice([]string{"2", "3", "4", "9"}),
						func(s1 string) error {
							counter++
							calledWith = append(calledWith, s1)
							return nil
						},
						func(leader string, follower string) error {
							// Both fields should be the same
							require.Equal(t, leader, follower)
							sameCalledWith = append(sameCalledWith, leader)
							equalCount++
							return nil
						},
						strings.Compare,
					), func() {
						require.Equal(t, 3, counter)
						require.Equal(t, []string{"1", "5", "8"}, calledWith)
						require.Equal(t, 2, equalCount)
						require.Equal(t, []string{"2", "4"}, sameCalledWith)
					}
			},
			wantErr: false,
		},
		{
			name: "errors are propagated",
			validate: func(t *testing.T) (*streamutils.ZipStreams[string, string], func()) {
				return streamutils.NewZipStreams[string, string](
						stream.Slice([]string{"1", "2", "5", "8"}),
						stream.Slice([]string{"2", "3", "9"}),
						func(s1 string) error {
							return trace.Errorf("something bad")
						},
						func(leader string, follower string) error {
							return nil
						},
						strings.Compare,
					), func() {
					}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z, validate := tt.validate(t)
			err := z.Process()
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			validate()
		})
	}
}

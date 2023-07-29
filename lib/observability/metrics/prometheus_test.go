/*
Copyright 2021 Gravitational, Inc.

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

package metrics

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func Test_labelGetter(t *testing.T) {
	ctxWithLabels := context.WithValue(context.Background(), promLabelsKey{}, prometheus.Labels{
		"foo": "bar",
	})
	type args struct {
		ctx   context.Context
		label string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "label exists",
			args: args{
				label: "foo",
				ctx:   ctxWithLabels,
			},
			want: "bar",
		},
		{
			name: "label doesn't exist",
			args: args{
				label: "not_found",
				ctx:   ctxWithLabels,
			},
			want: "unknown",
		},
		{
			name: "labels not set in ctx",
			args: args{
				label: "not_found",
				ctx:   context.Background(),
			},
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := labelGetter(tt.args.label)
			require.Equal(t, tt.want, got(tt.args.ctx))
		})
	}
}

/*

 Copyright 2022 Gravitational, Inc.

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

package reversetunnel

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func Test_remoteSite_getLocalWatchedCerts(t *testing.T) {
	tests := []struct {
		name           string
		clusterVersion string
		want           types.CertAuthorityFilter
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name:           "pre Database CA, only Host and User CA",
			clusterVersion: "9.0.0",
			want: types.CertAuthorityFilter{
				types.HostCA: "test",
				types.UserCA: "test",
			},
			errorAssertion: require.NoError,
		},
		{
			name:           "all certs should be returned",
			clusterVersion: "10.0.0",
			want: types.CertAuthorityFilter{
				types.DatabaseCA: "test",
				types.HostCA:     "test",
				types.UserCA:     "test",
			},
			errorAssertion: require.NoError,
		},
		{
			name:           "invalid version",
			clusterVersion: "foo",
			errorAssertion: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &remoteSite{
				srv: &server{
					Config: Config{
						ClusterName: "test",
					},
				},
				logger: log.NewEntry(utils.NewLoggerForTests()),
			}
			got, err := s.getLocalWatchedCerts(tt.clusterVersion)
			tt.errorAssertion(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}

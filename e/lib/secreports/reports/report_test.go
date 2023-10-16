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

package reports

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivilegeAccessReportUpdate(t *testing.T) {
	t.Parallel()

	vMap := map[string]string{
		"0.0.1": "87a1f05c51eb5766e4a02ae348f26051",
	}

	data, err := json.Marshal(&PrivilegeAccessReport)
	require.NoError(t, err)
	m := md5.New()
	_, err = m.Write(data)
	require.NoError(t, err)
	gotSum := hex.EncodeToString(m.Sum(nil))

	wantVersion, ok := vMap[PrivilegeAccessReport.Version]
	require.True(t, ok, "version %q not found", PrivilegeAccessReport.Version)
	// Check if report was updated and checksum is different.
	// For each report update a new version should be created.
	require.Equal(t, wantVersion, gotSum, "version %q checksum mismatch. Please update report version", PrivilegeAccessReport.Version)
}

/*
Copyright 2023 Gravitational, Inc.

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

package accesslist

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuditMarshaling(t *testing.T) {
	audit := Audit{
		Frequency: time.Hour,
	}

	data, err := json.Marshal(&audit)
	require.NoError(t, err)

	raw := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(data, &raw))

	require.Equal(t, "1h0m0s", raw["frequency"])
}

func TestAuditUnmarshaling(t *testing.T) {
	raw := map[string]interface{}{
		"frequency": "1h",
	}

	data, err := json.Marshal(&raw)
	require.NoError(t, err)

	var audit Audit
	require.NoError(t, json.Unmarshal(data, &audit))

	require.Equal(t, time.Hour, audit.Frequency)
}

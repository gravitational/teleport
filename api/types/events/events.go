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

package events

import "github.com/gogo/protobuf/proto"

func trimN(s string, n int) string {
	if n <= 0 {
		return s
	}
	if len(s) > n {
		return s[:n]
	}
	return s
}

func maxSizePerField(maxLength, customFields int) int {
	if customFields == 0 {
		return maxLength
	}
	return maxLength / customFields
}

// TrimToMaxSize trims the DatabaseSessionQuery message content. The maxSize is used to calculate
// per-filed max size where only user input message fields DatabaseQuery and DatabaseQueryParameters are taken into
// account.
func (m *DatabaseSessionQuery) TrimToMaxSize(maxSize int) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	out := proto.Clone(m).(*DatabaseSessionQuery)
	out.DatabaseQuery = ""
	out.DatabaseQueryParameters = nil

	// Use 10% max size ballast + message size without custom fields.
	sizeBallast := maxSize/10 + out.Size()
	maxSize -= sizeBallast

	// Check how many custom fields are set.
	customFieldsCount := 0
	if m.DatabaseQuery != "" {
		customFieldsCount++
	}
	for range m.DatabaseQueryParameters {
		customFieldsCount++
	}

	maxFieldsSize := maxSizePerField(maxSize, customFieldsCount)

	out.DatabaseQuery = trimN(m.DatabaseQuery, maxFieldsSize)
	if m.DatabaseQueryParameters != nil {
		out.DatabaseQueryParameters = make([]string, len(m.DatabaseQueryParameters))
	}
	for i, v := range m.DatabaseQueryParameters {
		out.DatabaseQueryParameters[i] = trimN(v, maxFieldsSize)
	}
	return out
}

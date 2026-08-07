// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package label

// LabelGetter allows retrieving a particular label by name or retreiving all
// labels at once. Prefer to use GetLabel when possible to avoid unnecessary
// copies.
type LabelGetter interface {
	GetLabel(key string) (value string, ok bool)
	GetAllLabels() map[string]string
}

type MapLabelGetter map[string]string

func (m MapLabelGetter) GetLabel(key string) (value string, ok bool) {
	v, ok := m[key]
	return v, ok
}

func (m MapLabelGetter) GetAllLabels() map[string]string {
	return map[string]string(m)
}

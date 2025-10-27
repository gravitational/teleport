/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { getResourcesSection } from 'teleport/Navigation/ResourcesSection';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

describe('getResourcesSection', () => {
  test('titles for subsections by kind are sorted', () => {
    const navSections = getResourcesSection({
      clusterId: 'cluster',
      preferences: makeDefaultUserPreferences(),
      updatePreferences: async () => {},
      searchParams: new URLSearchParams(),
    });

    const titles = navSections.subsections.map(s => s.title);

    // First two sections are fixed.
    const fixed = ['All Resources', 'Pinned Resources'];

    // Kind filters that should be sorted alphabetically.
    const kindFilters = [
      'Applications',
      'Databases',
      'Desktops',
      'Git Servers',
      'Kubernetes Clusters',
      'MCP Servers',
      'SSH Resources',
    ];

    const expected = [
      ...fixed,
      ...kindFilters.toSorted((a, b) => a.localeCompare(b)),
    ];

    expect(titles).toEqual(expected);
  });
});

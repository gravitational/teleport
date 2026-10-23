/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { render } from 'design/utils/testing';

import { SessionRecordingGrid } from './RecordingWithMetadata';

// Asserted through a child rather than the render container, which also holds
// the design system's theme script.
function renderGrid(sidebarHidden: boolean) {
  render(
    <SessionRecordingGrid sidebarHidden={sidebarHidden}>
      <div data-testid="area" />
    </SessionRecordingGrid>
  );

  return screen.getByTestId('area').parentElement;
}

describe('the recording grid', () => {
  it('spans the timeline when the sidebar is hidden', () => {
    expect(renderGrid(true)).toHaveStyle({
      gridTemplateAreas: `'recording' 'timeline'`,
      gridTemplateColumns: '1fr',
    });
  });

  it('seats the timeline beside the sidebar when it is shown', () => {
    expect(renderGrid(false)).toHaveStyle({
      gridTemplateAreas: `'sidebar recording' 'sidebar timeline'`,
    });
  });
});

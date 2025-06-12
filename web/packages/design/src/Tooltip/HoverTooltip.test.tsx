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

import userEvent from '@testing-library/user-event';
import { createRef } from 'react';

import { render, screen } from 'design/utils/testing';

import { HoverTooltip } from './HoverTooltip';

describe('showOnlyOnOverflow behavior', () => {
  const tooltipText = 'tip content';
  const triggerText = 'anchor';

  it('does not show tooltip when there is no overflow', async () => {
    render(
      <HoverTooltip showOnlyOnOverflow tipContent={tooltipText}>
        <div>{triggerText}</div>
      </HoverTooltip>
    );

    await userEvent.hover(screen.getByText(triggerText));

    expect(screen.queryByText(tooltipText)).not.toBeInTheDocument();
  });

  it('shows tooltip when child element overflows its container', async () => {
    const containerRef = createRef<HTMLDivElement>();
    const childRef = createRef<HTMLDivElement>();

    render(
      <div ref={containerRef}>
        <HoverTooltip showOnlyOnOverflow tipContent={tooltipText}>
          <div ref={childRef}>{triggerText}</div>
        </HoverTooltip>
      </div>
    );

    // Mock overflow: content is wider than container.
    // Since JSDOM does not measure or calculate the layout, we can only mock the properties.
    Object.defineProperty(childRef.current, 'scrollWidth', {
      configurable: true,
      get: () => 200,
    });

    Object.defineProperty(containerRef.current, 'offsetWidth', {
      configurable: true,
      get: () => 100,
    });

    await userEvent.hover(screen.getByText(triggerText));

    expect(await screen.findByText(tooltipText)).toBeInTheDocument();
  });
});

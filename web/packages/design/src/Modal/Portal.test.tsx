/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import { screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import Portal from './Portal';

describe('design/Modal/Portal', () => {
  test('container to be attached to body element', () => {
    const { container } = renderPortal({});
    const content = screen.getByTestId('content');
    expect(container).not.toContainElement(content);
    expect(document.body).toContainElement(screen.getByTestId('parent'));
  });

  test('container to be attached to custom element', () => {
    const customElement = document.createElement('div');
    renderPortal({ container: customElement });
    expect(screen.queryByTestId('content')).not.toBeInTheDocument();
    expect(customElement).toHaveTextContent('hello');
  });

  test('disable the portal behavior', () => {
    const { container } = renderPortal({ disablePortal: true });
    expect(container).toContainElement(screen.getByTestId('content'));
  });
});

function renderPortal(props) {
  return render(
    <div data-testid="parent">
      <Portal {...props}>
        <div data-testid="content">hello</div>
      </Portal>
    </div>
  );
}

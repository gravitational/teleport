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

import { copyToClipboard } from 'design/utils/copyToClipboard';
import { fireEvent, render, screen } from 'design/utils/testing';

import { CopyButton } from './CopyButton';

jest.mock('design/utils/copyToClipboard', () => {
  return {
    __esModule: true,
    copyToClipboard: jest.fn(),
  };
});

describe('CopyButton', () => {
  it('prevents parent elements from stealing clicks', () => {
    const parentClick = jest.fn();

    render(
      <div onClick={parentClick}>
        <CopyButton name="copy data" />
      </div>
    );

    fireEvent.click(screen.getByLabelText('copy'));

    expect(parentClick).toHaveBeenCalledTimes(0);
    expect(copyToClipboard).toHaveBeenCalledTimes(1);
  });
});

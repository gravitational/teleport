/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { fireEvent, screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import { Notification } from './Notification';

test('click on action button does not expand or collapse notification', async () => {
  const description = 'An error happened';

  render(
    <Notification
      item={{
        id: '865801ca',
        severity: 'error',
        content: {
          title: 'Warning',
          description,
          action: { content: 'Retry' },
        },
      }}
      onRemove={() => {}}
    />
  );

  fireEvent.click(screen.getByText('Retry'));

  // Check if the text still has the initial, "collapsed" style (look at shortTextCss).
  expect(screen.getByText(description)).toHaveStyleRule(
    '-webkit-line-clamp',
    '3'
  );
});

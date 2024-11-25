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

import { screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { render } from 'design/utils/testing';

import { ViewInPolicyButton } from 'teleport/Audit/EventList/ViewInPolicyButton';
import cfg from 'teleport/config';
import { RawEvents } from 'teleport/services/audit';

import makeEvent from '../../services/audit/makeEvent';

test('should not render if the event is not in the list of supported events', () => {
  const event: RawEvents['TOK005E'] = {
    code: 'TOK005E',
    event: 'okta.assignment.cleanup',
    name: '3x_0GLrBqnzIjqaXH2ho1G3H07_7NnneUVxPZ_q1Ji4',
    source: 'user-assignment-creator',
    time: '2024-11-13T13:32:11.397Z',
    uid: '4b9dde0c-4f1e-45d0-9a59-d970f7d28f16',
    user: 'sasha',
  };

  render(<ViewInPolicyButton event={makeEvent(event)} />);

  expect(screen.queryByRole('link')).not.toBeInTheDocument();
});

test('should render a link for access path changes', () => {
  const event: RawEvents['TAG001I'] = {
    affected_resource_name: '2k6sycjspmhaib',
    affected_resource_source: 'TELEPORT',
    affected_resource_kind: 'server',
    user: '',
    change_id: 'f6be68d1-fa5d-4ff7-ad0b-5c1447e251a0',
    code: 'TAG001I',
    event: 'access_graph.access_path_changed',
    time: '2024-11-13T13:53:29.983Z',
    uid: '22c49326-2b72-4503-bd62-dac5ac610be6',
  };

  render(
    <MemoryRouter>
      <ViewInPolicyButton event={makeEvent(event)} />
    </MemoryRouter>
  );

  const link = screen.getByRole('link');

  expect(link).toBeInTheDocument();

  expect(link).toHaveProperty(
    'href',
    `http://localhost${cfg.getAccessGraphCrownJewelAccessPathUrl(event.change_id)}`
  );
});

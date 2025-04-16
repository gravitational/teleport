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

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import 'jest-canvas-mock';

import { allAccessAcl } from 'teleport/mocks/contexts';

import DocumentSsh from '.';
import ConsoleContext from '../consoleContext';
import ConsoleContextProvider from '../consoleContextProvider';

test('file transfer buttons are disabled if user does not have access', async () => {
  const ctx = new ConsoleContext();
  ctx.storeUser.setState({
    acl: { ...allAccessAcl, fileTransferAccess: false },
  });
  render(<Component ctx={ctx} />);
  expect(screen.getByTitle('Download files')).toBeDisabled();
});

test('file transfer buttons are enabled if user has access', async () => {
  const ctx = new ConsoleContext();
  ctx.storeUser.setState({
    acl: allAccessAcl,
  });
  render(<Component ctx={ctx} />);
  expect(screen.getByTitle('Download files')).toBeEnabled();
});

function Component({ ctx }: { ctx: ConsoleContext }) {
  return (
    <MemoryRouter>
      <ConsoleContextProvider value={ctx}>
        <DocumentSsh
          doc={{
            id: 123,
            status: 'connected',
            kind: 'terminal',
            serverId: '123',
            login: 'tester',
            latency: { client: 123, server: 2 },
            url: 'http://localhost',
            created: new Date(),
          }}
          visible={true}
        />
      </ConsoleContextProvider>
    </MemoryRouter>
  );
}

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

import { MemoryRouter } from 'react-router-dom';

import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { allAccessAcl } from 'teleport/mocks/contexts';
import TeleportContext from 'teleport/teleportContext';

import { ConfigureBot } from './ConfigureBot';
import { GitHubFlowProvider } from './useGitHubFlow';

describe('addBotToWorkflow Component', () => {
  const setup = () => {
    const ctx = new TeleportContext();

    ctx.storeUser.setState({
      username: 'joe@example.com',
      acl: allAccessAcl,
    });
    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <GitHubFlowProvider>
            <ConfigureBot />
          </GitHubFlowProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  it('does not display the button to go back', () => {
    setup();
    expect(screen.queryByTestId('button-back')).not.toBeInTheDocument();
  });

  it('displays the button to finish', () => {
    setup();
    expect(screen.getByTestId('button-next')).toBeInTheDocument();
  });
});

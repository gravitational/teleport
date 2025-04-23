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
import cfg from 'teleport/config';
import TeleportContext from 'teleport/teleportContext';

import { Finish } from './Finish';
import { GitHubFlowProvider, initialBotState } from './useGitHubFlow';

describe('finish Component', () => {
  const setup = ({ botName }) => {
    const ctx = new TeleportContext();
    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <GitHubFlowProvider bot={{ ...initialBotState, botName }}>
            <Finish />
          </GitHubFlowProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  it('renders with dynamic content based on hook', () => {
    setup({ botName: 'test-bot' });
    expect(
      screen.getByText('Your Bot is Added to Teleport')
    ).toBeInTheDocument();
    expect(screen.getByText('View Bots')).toBeInTheDocument();
    expect(screen.getByText('Add Another Bot')).toBeInTheDocument();
  });

  it('has correct links on buttons', () => {
    setup({ botName: 'test-bot' });
    expect(screen.getByText('View Bots').closest('a')).toHaveAttribute(
      'href',
      cfg.getBotsRoute()
    );
    expect(screen.getByText('Add Another Bot').closest('a')).toHaveAttribute(
      'href',
      cfg.getBotsNewRoute()
    );
  });
});

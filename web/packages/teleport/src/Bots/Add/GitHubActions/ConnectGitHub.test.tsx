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

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { allAccessAcl } from 'teleport/mocks/contexts';
import TeleportContext from 'teleport/teleportContext';

import { ConnectGitHub } from './ConnectGitHub';
import { GitHubFlowProvider } from './useGitHubFlow';

describe('connectGitHub Component', () => {
  function setup() {
    const ctx = new TeleportContext();

    ctx.storeUser.setState({
      username: 'joe@example.com',
      acl: allAccessAcl,
    });

    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <GitHubFlowProvider>
            <ConnectGitHub />
          </GitHubFlowProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  }

  // Test for default rendering
  it('renders basic elements', () => {
    setup();
    expect(
      screen.getByText('Step 2: Input Your GitHub Account Info')
    ).toBeInTheDocument();
    expect(screen.getByText('Next')).toBeInTheDocument();
    expect(screen.getByText('Back')).toBeInTheDocument();

    // rule form
    expect(screen.getByText('GitHub Repository Access:')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('https://github.com/gravitational/teleport')
    ).toBeInTheDocument();
    expect(screen.getByText('Git Ref')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('main')).toBeInTheDocument();
    expect(screen.getByText('Ref Type')).toBeInTheDocument();
    expect(
      screen.getByText('Name of the GitHub Actions Workflow')
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText('cd')).toBeInTheDocument();
    expect(screen.getByText('Environment')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('development')).toBeInTheDocument();
    expect(screen.getByText('Restrict to a GitHub User')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('octocat')).toBeInTheDocument();
  });
});

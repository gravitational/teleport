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

import { render, screen, userEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import * as botService from 'teleport/services/bot/bot';
import TeleportContext from 'teleport/teleportContext';

import { GitHubActions } from './GitHubActions';
import { GitHubFlowProvider } from './useGitHubFlow';

const tokenName = 'generated-test-token';
const authVersion = 'v15.0.0';

describe('gitHub component', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  type SetupProps = {
    hasAccess?: boolean;
  };

  const setup = ({ hasAccess = true }: SetupProps) => {
    const ctx = new TeleportContext();
    ctx.storeUser.setState({
      username: 'joe@example.com',
      acl: hasAccess ? allAccessAcl : { ...allAccessAcl, bots: noAccess },
      cluster: {
        authVersion: authVersion,
        clusterId: 'cluster-id',
        connectedText: 'connected-text',
        lastConnected: new Date('2023-01-01'),
        proxyVersion: 'v15.0.0',
        publicURL: 'publicurl',
        status: 'ok',
        url: 'url',
      },
    });

    jest.spyOn(ctx.resourceService, 'createRole').mockResolvedValue({
      id: 'role-id',
      kind: 'role',
      name: 'role-name',
      content: '',
    });
    jest.spyOn(ctx.joinTokenService, 'fetchJoinToken').mockResolvedValue({
      id: tokenName,
      expiry: new Date('2020-01-01'),
      safeName: '',
      isStatic: false,
      method: 'kubernetes',
      roles: [],
      content: '',
    });
    jest.spyOn(botService, 'createBot').mockResolvedValue();
    jest.spyOn(botService, 'createBotToken').mockResolvedValue({
      integrationName: 'test-integration',
      joinMethod: 'github',
      webFlowLabel: 'github-actions-ssh',
    });
    jest.spyOn(botService, 'getBot').mockResolvedValue(null);

    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <GitHubFlowProvider>
            <GitHubActions />
          </GitHubFlowProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  it('renders initial state with warning if user has no access', () => {
    setup({ hasAccess: false });
    expect(screen.getByText(/Insufficient permissions/)).toBeInTheDocument();
    expect(screen.getByTestId('button-next')).toBeDisabled();
    expect(screen.getByTestId('button-back-first-step')).toBeEnabled();
  });

  it('renders initial state with no warnings if user the necessary access', () => {
    setup({});
    expect(
      screen.queryByText(/Insufficient permissions/)
    ).not.toBeInTheDocument();
    expect(screen.getByTestId('button-next')).toBeEnabled();
  });

  it('allows the user to go through the whole flow', async () => {
    setup({});
    expect(
      screen.getByText(/Step 1: Scope the Permissions for Your Bot/)
    ).toBeInTheDocument();
    // fill up the forms and go through the flow
    // step 1: Configure Bot Access
    const botNameInput = screen.getByPlaceholderText('github-actions-cd');
    await userEvent.type(botNameInput, 'bot-name');
    const sshUserInput = screen.getByPlaceholderText('ubuntu');
    await userEvent.type(sshUserInput, 'ssh-user');
    await userEvent.click(screen.getByTestId('button-next'));
    // step 2: Connect GitHub
    expect(
      screen.getByText(/Step 2: Input Your GitHub Account Info/)
    ).toBeInTheDocument();
    const repositoryInput = screen.getByPlaceholderText(
      'https://github.com/gravitational/teleport'
    );
    await userEvent.type(repositoryInput, 'https://github.com/owner/repo');
    await userEvent.click(screen.getByTestId('button-next'));
    // step 3: Add Bot to GitHub
    expect(
      screen.getByText(/Step 3: Connect Your Bot in a GitHub Actions Workflow/)
    ).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('button-next'));
    // Finish screen
    expect(
      screen.getByText(/Your Bot is Added to Teleport/)
    ).toBeInTheDocument();
    expect(screen.getByText(/View Bots/)).toBeInTheDocument();
    expect(screen.getByText(/Add Another Bot/)).toBeInTheDocument();
  });
});

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

import { render, screen, userEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { allAccessAcl } from 'teleport/mocks/contexts';
import * as botService from 'teleport/services/bot/bot';
import { Access, Acl } from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';

import { ConfigureBot } from './ConfigureBot';
import { GitHubFlowProvider } from './useGitHubFlow';

type SetupProps = {
  access?: Acl;
};
const noAccessAcl: Access = {
  list: false,
  read: false,
  edit: false,
  create: false,
  remove: false,
};

describe('configureBot Component', () => {
  const setup = ({ access }: SetupProps) => {
    const ctx = new TeleportContext();

    ctx.storeUser.setState({
      username: 'joe@example.com',
      acl: access || allAccessAcl,
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

    return ctx;
  };

  it('renders the necessary input fields', () => {
    setup({});
    expect(screen.getByPlaceholderText('ubuntu')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('github-actions-cd')
    ).toBeInTheDocument();
  });

  it('shows an alert if the user lacks sufficient permissions bots', () => {
    setup({ access: { ...allAccessAcl, bots: noAccessAcl } });
    expect(
      screen.getByText(
        'Insufficient permissions. In order to create a bot, you need permissions to create roles, bots and join tokens.'
      )
    ).toBeInTheDocument();
    expect(screen.getByTestId('button-next')).toBeDisabled();
  });

  it('shows an alert if the user lacks sufficient permissions tokens', () => {
    setup({ access: { ...allAccessAcl, tokens: noAccessAcl } });
    expect(
      screen.getByText(
        'Insufficient permissions. In order to create a bot, you need permissions to create roles, bots and join tokens.'
      )
    ).toBeInTheDocument();
    expect(screen.getByTestId('button-next')).toBeDisabled();
  });

  it('shows an alert if the user lacks sufficient permissions roles', () => {
    setup({ access: { ...allAccessAcl, roles: noAccessAcl } });
    expect(
      screen.getByText(
        'Insufficient permissions. In order to create a bot, you need permissions to create roles, bots and join tokens.'
      )
    ).toBeInTheDocument();
    expect(screen.getByTestId('button-next')).toBeDisabled();
  });

  it('shows an error if the bot name already exists', async () => {
    setup({});
    jest.spyOn(botService, 'getBot').mockResolvedValue({
      traits: [],
      kind: 'GitHub Actions',
      name: 'bot-name',
      roles: ['bot-github-actions-bot'],
      namespace: '',
      description: '',
      labels: null,
      revision: '',
      status: '',
      subKind: '',
      version: '',
    });

    const botNameInput = screen.getByPlaceholderText('github-actions-cd');
    await userEvent.type(botNameInput, 'bot-name');
    const sshUserInput = screen.getByPlaceholderText('ubuntu');
    await userEvent.type(sshUserInput, 'ssh-user');
    await userEvent.click(screen.getByTestId('button-next'));
    expect(
      screen.getByText(
        'A bot with this name already exist, please use a different name.'
      )
    ).toBeInTheDocument();
  });

  it('shows an error if the bot name is empty or contains whitespaces', async () => {
    setup({});

    const botNameInput = screen.getByPlaceholderText('github-actions-cd');

    // empty
    await userEvent.type(botNameInput, ' ');
    await userEvent.click(screen.getByTestId('button-next'));
    expect(
      screen.getByText('Name for the Bot Workflow is required')
    ).toBeInTheDocument();

    // whitespaces
    await userEvent.type(botNameInput, 'my bot');
    await userEvent.click(screen.getByTestId('button-next'));
    expect(
      screen.getByText(
        'Special characters are not allowed in the workflow name, please use name composed only from characters, hyphens, dots, and plus signs'
      )
    ).toBeInTheDocument();
  });
});

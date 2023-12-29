import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import TeleportContext from 'teleport/teleportContext';
import { allAccessAcl } from 'teleport/mocks/contexts';

import { Access, Acl } from 'teleport/services/user';

import { GitHubFlowProvider } from './useGitHubFlow';
import { ConfigureBot } from './ConfigureBot';

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
  };

  it('renders the necessary input fields', () => {
    setup({});
    expect(screen.getByPlaceholderText('ex. ubuntu')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('ex. github-actions-cd')
    ).toBeInTheDocument();
  });

  it('shows an alert if the user lacks sufficient permissions bots', () => {
    setup({ access: { ...allAccessAcl, bots: noAccessAcl } });
    expect(
      screen.getByText(
        'Insufficient permissions. In order to create a bot, you need permissions to create roles, bots and join tokens.'
      )
    ).toBeInTheDocument();
  });

  it('shows an alert if the user lacks sufficient permissions tokens', () => {
    setup({ access: { ...allAccessAcl, tokens: noAccessAcl } });
    expect(
      screen.getByText(
        'Insufficient permissions. In order to create a bot, you need permissions to create roles, bots and join tokens.'
      )
    ).toBeInTheDocument();
  });

  it('shows an alert if the user lacks sufficient permissions roles', () => {
    setup({ access: { ...allAccessAcl, roles: noAccessAcl } });
    expect(
      screen.getByText(
        'Insufficient permissions. In order to create a bot, you need permissions to create roles, bots and join tokens.'
      )
    ).toBeInTheDocument();
  });
});

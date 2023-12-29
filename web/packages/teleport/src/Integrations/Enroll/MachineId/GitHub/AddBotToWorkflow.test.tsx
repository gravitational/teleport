import React from 'react';
import { render, screen } from 'design/utils/testing';
import { MemoryRouter } from 'react-router-dom';

import { ContextProvider } from 'teleport';
import TeleportContext from 'teleport/teleportContext';
import { allAccessAcl } from 'teleport/mocks/contexts';

import { GitHubFlowProvider } from './useGitHubFlow';
import { ConfigureBot } from './ConfigureBot';

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

import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import TeleportContext from 'teleport/teleportContext';
import cfg from 'teleport/config';

import { GitHubFlowProvider, initialBotState } from './useGitHubFlow';
import { Finish } from './Finish';

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
      screen.getByText('Your Machine User is Added to Teleport')
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        `Machine User test-bot has been successfully added to this Teleport Cluster. You can see bot-test-bot in the Teleport Users page and you can always find the sample GitHub Actions workflow again from the machine user's options.`
      )
    ).toBeInTheDocument();
    expect(screen.getByText('View Machine Users')).toBeInTheDocument();
    expect(screen.getByText('Add Another Integration')).toBeInTheDocument();
  });

  it('has correct links on buttons', () => {
    setup({ botName: 'test-bot' });
    expect(screen.getByText('View Machine Users').closest('a')).toHaveAttribute(
      'href',
      cfg.routes.users
    );
    expect(
      screen.getByText('Add Another Integration').closest('a')
    ).toHaveAttribute('href', cfg.getIntegrationEnrollRoute(null));
  });
});

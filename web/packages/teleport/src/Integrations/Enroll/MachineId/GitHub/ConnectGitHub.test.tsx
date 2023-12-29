import React from 'react';

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { allAccessAcl } from 'teleport/mocks/contexts';

import { ContextProvider } from 'teleport';
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
      screen.getByPlaceholderText(
        'ex. https://github.com/gravitational/teleport'
      )
    ).toBeInTheDocument();
    expect(screen.getByText('Git Ref')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('main')).toBeInTheDocument();
    expect(screen.getByText('Ref Type')).toBeInTheDocument();
    expect(
      screen.getByText('Name of the GitHub Actions Workflow')
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText('ex. cd')).toBeInTheDocument();
    expect(screen.getByText('Environmnet')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('ex. development')).toBeInTheDocument();
    expect(screen.getByText('Restrict to a GitHub User')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('ex. octocat')).toBeInTheDocument();
  });
});

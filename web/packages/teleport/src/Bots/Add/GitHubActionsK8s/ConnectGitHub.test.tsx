/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { QueryClientProvider } from '@tanstack/react-query';
import { PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { render, screen, testQueryClient } from 'design/utils/testing';

import { ConnectGitHub } from './ConnectGitHub';
import { GitHubK8sFlowProvider, useGitHubK8sFlow } from './useGitHubK8sFlow';

describe('ConnectGitHub', () => {
  test('renders', async () => {
    renderComponent({
      allowAnyBranch: false,
      branch: 'main',
      enterpriseJwks: '{"keys":[]}',
      enterpriseSlug: 'octo-enterprise',
      environment: 'production',
      gitHubUrl: 'https://github.com/gravitational/teleport',
      isBranchDisabled: false,
      ref: 'main',
      refType: 'branch',
      workflow: 'my-workflow',
    });

    expect(
      screen.getByRole('heading', { name: 'Connect to GitHub' })
    ).toBeInTheDocument();

    expect(
      screen.getByPlaceholderText('https://github.com/gravitational/teleport')
    ).toHaveValue('https://github.com/gravitational/teleport');

    expect(screen.getByPlaceholderText('main')).toHaveValue('main');

    expect(screen.getByPlaceholderText('my-workflow')).toHaveValue(
      'my-workflow'
    );

    expect(screen.getByPlaceholderText('production')).toHaveValue('production');

    expect(screen.getByPlaceholderText('ref/heads/main')).toHaveValue('main');

    expect(screen.getByPlaceholderText('octo-enterprise')).toHaveValue(
      'octo-enterprise'
    );

    expect(screen.getByPlaceholderText('{"keys":[ --snip-- ]}')).toHaveValue(
      '{"keys":[]}'
    );
  });
});

function renderComponent(
  initialState?: ReturnType<typeof useGitHubK8sFlow>['state']
) {
  return {
    ...render(<ConnectGitHub />, { wrapper: makeWrapper(initialState) }),
  };
}

function makeWrapper(
  initialState?: ReturnType<typeof useGitHubK8sFlow>['state']
) {
  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          <GitHubK8sFlowProvider intitialState={initialState}>
            {children}
          </GitHubK8sFlowProvider>
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    );
  };
}

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
import { createMemoryHistory } from 'history';
import { PropsWithChildren } from 'react';
import { Router } from 'react-router';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { BotFlowType } from 'teleport/Bots/types';
import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';

import { GitHubActionsK8s } from './GitHubActionsK8s';

describe('GitHubActionsK8s', () => {
  test('complete flow: minimal', async () => {
    const { user, history } = renderComponent();
    const replaceMock = jest.spyOn(history, 'replace');

    expect(
      screen.getByRole('heading', { name: 'GitHub Actions + Kubernetes' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Start' }));

    expect(
      screen.getByRole('heading', { name: 'Connect to GitHub' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Next' }));

    expect(screen.getByText('Repository is required')).toBeInTheDocument();
    expect(screen.getByText('A branch is required')).toBeInTheDocument();

    await user.type(
      screen.getByLabelText('Repository URL'),
      'https://github.com/gravitational/teleport'
    );

    await user.type(screen.getByLabelText('Branch'), 'main');

    await user.click(screen.getByRole('button', { name: 'Next' }));

    expect(
      screen.getByRole('heading', { name: 'Configure Access' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Next' }));

    expect(
      screen.getByRole('heading', { name: 'Setup Workflow' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Finish' }));

    expect(replaceMock).toHaveBeenLastCalledWith('/web/bots');
  });
});

function renderComponent() {
  const user = userEvent.setup();
  const history = createMemoryHistory({
    initialEntries: [cfg.getBotsNewRoute(BotFlowType.GitHubActionsSsh)],
  });
  return {
    ...render(<GitHubActionsK8s />, {
      wrapper: makeWrapper({ history }),
    }),
    user,
    history,
  };
}

function makeWrapper(opts: {
  history: ReturnType<typeof createMemoryHistory>;
}) {
  const { history } = opts;
  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          <InfoGuidePanelProvider>
            <ContentMinWidth>
              <Router history={history}>{children}</Router>
            </ContentMinWidth>
          </InfoGuidePanelProvider>
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    );
  };
}

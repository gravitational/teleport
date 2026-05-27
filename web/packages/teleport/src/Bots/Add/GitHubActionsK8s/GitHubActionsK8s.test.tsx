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
import selectEvent from 'react-select-event';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  act,
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { BotFlowType } from 'teleport/Bots/types';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';
import { fetchUnifiedResourcesSuccess } from 'teleport/test/helpers/resources';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { trackingTester } from '../Shared/trackingTester';
import { TrackingProvider } from '../Shared/useTracking';
import { GitHubActionsK8sWithoutTracking } from './GitHubActionsK8s';

enableMswServer();

beforeEach(() => {
  server.use(genWizardCiCdSuccess());
  server.use(userEventCaptureSuccess());
  server.use(fetchUnifiedResourcesSuccess());

  jest.useFakeTimers();
});

afterAll(() => {
  jest.useRealTimers();
  jest.resetAllMocks();
});

describe('GitHubActionsK8s', () => {
  test('complete flow: minimal', async () => {
    const tracking = trackingTester();

    const { user, history, unmount } = renderComponent({
      trackingEventId: 'test-tracking-event-id',
    });
    const replaceMock = jest.spyOn(history, 'replace');

    tracking.assertStart('test-tracking-event-id');

    expect(
      screen.getByRole('heading', { name: 'GitHub Actions + Kubernetes' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Start' }));

    tracking.assertStep(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_WELCOME',
      'INTEGRATION_ENROLL_STATUS_CODE_SUCCESS'
    );

    expect(
      screen.getByRole('heading', { name: 'Connect to GitHub' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Next' }));

    tracking.assertError(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'validation error'
    );

    expect(screen.getByText('Repository is required')).toBeInTheDocument();
    expect(screen.getByText('A branch is required')).toBeInTheDocument();

    await user.type(
      screen.getByLabelText('Repository URL'),
      'https://github.com/gravitational/teleport'
    );

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_REPOSITORY_URL'
    );

    await user.type(screen.getByLabelText('Branch'), 'main');

    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_BRANCH'
    );

    await user.click(screen.getByRole('button', { name: 'Next' }));

    tracking.assertStep(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_STATUS_CODE_SUCCESS'
    );

    expect(
      screen.getByRole('heading', { name: 'Configure Access' })
    ).toBeInTheDocument();

    const input = screen.getByLabelText('Kubernetes Groups');
    await user.type(input, 'viewers{enter}');

    await user.click(screen.getByRole('button', { name: 'Next' }));

    tracking.assertStep(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONFIGURE_ACCESS',
      'INTEGRATION_ENROLL_STATUS_CODE_SUCCESS'
    );

    expect(
      screen.getByRole('heading', { name: 'Set Up Workflow' })
    ).toBeInTheDocument();

    const select = screen.getByLabelText('Select a cluster to access');
    await selectEvent.select(select, 'kube-lon-staging-01.example.com');

    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_SETUP_WORKFLOW',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_KUBERNETES_CLUSTER_NAME'
    );

    await user.click(screen.getByRole('button', { name: 'Close' }));

    await user.click(screen.getByRole('button', { name: 'Confirm' }));

    tracking.assertStep(
      'test-tracking-event-id',
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_SETUP_WORKFLOW',
      'INTEGRATION_ENROLL_STATUS_CODE_SUCCESS'
    );

    expect(replaceMock).toHaveBeenLastCalledWith('/web/bots');

    unmount();

    tracking.assertComplete('test-tracking-event-id');
  });
});

function renderComponent(opts?: { trackingEventId?: string }) {
  const { trackingEventId } = opts ?? {};
  const user = userEvent.setup({
    advanceTimers: t => jest.advanceTimersByTime(t),
  });
  const history = createMemoryHistory({
    initialEntries: [cfg.getBotsNewRoute(BotFlowType.GitHubActionsSsh)],
  });
  return {
    ...render(<GitHubActionsK8sWithoutTracking />, {
      wrapper: makeWrapper({ history, trackingEventId }),
    }),
    user,
    history,
  };
}

function makeWrapper(opts: {
  history: ReturnType<typeof createMemoryHistory>;
  trackingEventId?: string;
}) {
  const { history, trackingEventId } = opts;
  const ctx = createTeleportContext();

  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ContextProvider ctx={ctx}>
          <ConfiguredThemeProvider theme={darkTheme}>
            <InfoGuidePanelProvider>
              <ContentMinWidth>
                <TrackingProvider initialEventId={trackingEventId}>
                  <Router history={history}>{children}</Router>
                </TrackingProvider>
              </ContentMinWidth>
            </InfoGuidePanelProvider>
          </ConfiguredThemeProvider>
        </ContextProvider>
      </QueryClientProvider>
    );
  };
}

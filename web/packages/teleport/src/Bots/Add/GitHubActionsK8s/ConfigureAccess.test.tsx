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
import { setupServer } from 'msw/node';
import { PropsWithChildren } from 'react';
import selectEvent from 'react-select-event';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  act,
  render,
  screen,
  testQueryClient,
  userEvent,
  within,
} from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';
import { fetchUnifiedResourcesSuccess } from 'teleport/test/helpers/resources';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { trackingTester } from '../Shared/trackingTester';
import { TrackingProvider } from '../Shared/useTracking';
import { ConfigureAccess } from './ConfigureAccess';
import { GitHubK8sFlowProvider, useGitHubK8sFlow } from './useGitHubK8sFlow';

const server = setupServer();

beforeAll(() => {
  server.listen();

  // Basic mock for all tests
  server.use(genWizardCiCdSuccess());
  server.use(fetchUnifiedResourcesSuccess());
  server.use(userEventCaptureSuccess());

  jest.useFakeTimers();
});

afterAll(() => {
  server.close();

  jest.useRealTimers();
  jest.resetAllMocks();
});

describe('ConfigureAccess', () => {
  test('renders', async () => {
    renderComponent({
      initialState: {
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
        kubernetesGroups: ['viewers'],
        kubernetesLabels: [{ name: '*', values: ['*'] }],
        kubernetesUsers: ['user@example.com'],
      },
    });

    expect(
      screen.getByRole('heading', { name: 'Configure Access' })
    ).toBeInTheDocument();

    expect(screen.getByLabelText('Kubernetes Groups')).toBeInTheDocument();
    expect(screen.getByLabelText('Labels')).toBeInTheDocument();
    expect(screen.getByLabelText('Kubernetes Users')).toBeInTheDocument();
  });

  test('navigation', async () => {
    const { onNextStep, onPrevStep, user } = renderComponent({
      initialState: {
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
        kubernetesGroups: [],
        kubernetesLabels: [{ name: '*', values: ['*'] }],
        kubernetesUsers: [],
      },
    });

    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(onNextStep).toHaveBeenCalledTimes(1);
    expect(onPrevStep).toHaveBeenCalledTimes(0);

    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(onNextStep).toHaveBeenCalledTimes(1);
    expect(onPrevStep).toHaveBeenCalledTimes(1);
  });

  test('input groups', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Kubernetes Groups');
    await selectEvent.create(input, 'system:masters');
    await user.type(input, 'viewers{enter}');

    expect(screen.getByText('system:masters')).toBeInTheDocument();
    expect(screen.getByText('viewers')).toBeInTheDocument();

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONFIGURE_ACCESS',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_KUBERNETES_GROUPS'
    );
  });

  test('input labels', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Labels');
    await user.click(within(input).getByRole('button'));

    const modal = screen.getByTestId('Modal');
    const manualInput = within(modal).getByPlaceholderText('name: value');
    await user.type(manualInput, 'foo:bar{enter}');
    await user.click(within(modal).getByRole('button', { name: 'Done' }));

    expect(modal).not.toBeInTheDocument();
    expect(screen.getByText('foo: bar')).toBeInTheDocument();

    // Skip start event
    tracking.skip();
    // Skip section event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONFIGURE_ACCESS',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_KUBERNETES_LABELS'
    );
  });

  test('input users', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Kubernetes Users');
    await selectEvent.create(input, 'user1@example.com');
    await user.type(input, 'user2@example.com{enter}');

    expect(screen.getByText('user1@example.com')).toBeInTheDocument();
    expect(screen.getByText('user2@example.com')).toBeInTheDocument();

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONFIGURE_ACCESS',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_KUBERNETES_USERS'
    );
  });
});

function renderComponent(opts?: {
  initialState?: ReturnType<typeof useGitHubK8sFlow>['state'];
  disableTracking?: boolean;
}) {
  const user = userEvent.setup({
    advanceTimers: t => jest.advanceTimersByTime(t),
  });
  const onNextStep = jest.fn();
  const onPrevStep = jest.fn();
  return {
    ...render(<ConfigureAccess nextStep={onNextStep} prevStep={onPrevStep} />, {
      wrapper: makeWrapper(opts),
    }),
    user,
    onNextStep,
    onPrevStep,
  };
}

function makeWrapper(opts?: {
  initialState?: ReturnType<typeof useGitHubK8sFlow>['state'];
  disableTracking?: boolean;
}) {
  const { initialState, disableTracking = false } = opts ?? {};
  const ctx = createTeleportContext();

  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ContextProvider ctx={ctx}>
          <ConfiguredThemeProvider theme={darkTheme}>
            <TrackingProvider disabled={disableTracking}>
              <GitHubK8sFlowProvider intitialState={initialState}>
                {children}
              </GitHubK8sFlowProvider>
            </TrackingProvider>
          </ConfiguredThemeProvider>
        </ContextProvider>
      </QueryClientProvider>
    );
  };
}

/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';
import { fetchUnifiedResourcesSuccess } from 'teleport/test/helpers/resources';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { trackingTester } from '../Shared/trackingTester';
import { TrackingProvider } from '../Shared/useTracking';
import { Finish } from './Finish';
import { GitHubK8sFlowProvider, useGitHubK8sFlow } from './useGitHubK8sFlow';

// Switching the async component for its sync equivalent - the latter is a pain
// to test without getting act() warnings and errors.
jest.mock('shared/components/FieldSelect/FieldSelectCreatable', () => {
  const actual = jest.requireActual(
    'shared/components/FieldSelect/FieldSelectCreatable'
  );
  return {
    ...actual,
    FieldSelectCreatableAsync: (props: {
      loadOptions?: unknown;
      defaultOptions?: unknown;
    }) => {
      const {
        // eslint-disable-next-line unused-imports/no-unused-vars
        loadOptions,
        // eslint-disable-next-line unused-imports/no-unused-vars
        defaultOptions,
        ...rest
      } = props;
      return <actual.FieldSelectCreatable {...rest} />;
    },
  };
});

enableMswServer();

beforeEach(() => {
  // Basic mock for all tests
  server.use(genWizardCiCdSuccess());
  server.use(fetchUnifiedResourcesSuccess());
  server.use(userEventCaptureSuccess());

  jest.useFakeTimers();
});

afterAll(() => {
  jest.useRealTimers();
  jest.resetAllMocks();
});

describe('Finish', () => {
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
        kubernetesCluster: 'my-kubernetes-cluster',
      },
    });

    expect(
      screen.getByRole('heading', { name: 'Set Up Workflow' })
    ).toBeInTheDocument();

    expect(
      screen.getByLabelText('Select a cluster to access')
    ).toBeInTheDocument();
    expect(screen.getByText('To complete the setup')).toBeInTheDocument();
  });

  test('cluster', async () => {
    const tracking = trackingTester();

    renderComponent();

    const input = screen.getByLabelText('Select a cluster to access');
    await selectEvent.create(input, 'test-cluster', {
      createOptionText: 'Use cluster "test-cluster"',
    });

    expect(screen.getByText('test-cluster')).toBeInTheDocument();

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_SETUP_WORKFLOW',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_KUBERNETES_CLUSTER_NAME'
    );
  });

  test('validates that a cluster is provided', async () => {
    const { onNextStep, user } = renderComponent({
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
        kubernetesUsers: [],
        kubernetesCluster: '',
      },
    });

    await user.click(screen.getByRole('button', { name: 'Close' }));

    expect(onNextStep).not.toHaveBeenCalled();
    expect(
      screen.getAllByText('A Kubernetes cluster is required')
    ).toHaveLength(1);
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
    ...render(<Finish nextStep={onNextStep} prevStep={onPrevStep} />, {
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

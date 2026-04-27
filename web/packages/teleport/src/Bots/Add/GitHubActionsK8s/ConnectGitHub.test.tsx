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
import { ComponentProps, PropsWithChildren } from 'react';
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

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';
import { fetchUnifiedResourcesSuccess } from 'teleport/test/helpers/resources';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { trackingTester } from '../Shared/trackingTester';
import { TrackingProvider } from '../Shared/useTracking';
import { ConnectGitHub } from './ConnectGitHub';
import { GitHubK8sFlowProvider } from './useGitHubK8sFlow';

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

describe('ConnectGitHub', () => {
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
        ref: 'refs/heads/main',
        refType: 'branch',
        workflow: 'my-workflow',
        kubernetesGroups: [],
        kubernetesLabels: [{ name: '*', values: ['*'] }],
        kubernetesUsers: [],
        kubernetesCluster: '',
      },
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

    expect(screen.getByPlaceholderText('refs/heads/main')).toHaveValue(
      'refs/heads/main'
    );

    expect(screen.getByPlaceholderText('octo-enterprise')).toHaveValue(
      'octo-enterprise'
    );

    expect(screen.getByPlaceholderText('{"keys":[ --snip-- ]}')).toHaveValue(
      '{"keys":[]}'
    );
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
        ref: 'refs/heads/main',
        refType: 'branch',
        workflow: 'my-workflow',
        kubernetesGroups: [],
        kubernetesLabels: [{ name: '*', values: ['*'] }],
        kubernetesUsers: [],
        kubernetesCluster: '',
      },
    });

    await user.click(screen.getByRole('button', { name: 'Next' }));
    expect(onNextStep).toHaveBeenCalledTimes(1);
    expect(onPrevStep).toHaveBeenCalledTimes(0);

    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(onNextStep).toHaveBeenCalledTimes(1);
    expect(onPrevStep).toHaveBeenCalledTimes(1);
  });

  test('input github url', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Repository URL');
    await user.type(input, 'https://github.com/foo/bar');

    expect(input).toHaveValue('https://github.com/foo/bar');

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_REPOSITORY_URL'
    );
  });

  test('input branch', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Branch');
    await user.type(input, 'main');

    expect(input).toHaveValue('main');
    expect(screen.getByLabelText('Git Ref')).toHaveValue('refs/heads/main');

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_BRANCH'
    );
  });

  test('toggle allow any branch', async () => {
    const { user } = renderComponent({ disableTracking: true });

    const input = screen.getByLabelText('Branch');
    await user.type(input, 'main');

    const check = screen.getByLabelText('Allow any branch');
    await user.click(check);

    expect(input).toHaveValue('');
  });

  test('input workflow', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Workflow');
    await user.type(input, 'my-workflow');

    expect(input).toHaveValue('my-workflow');

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_WORKFLOW'
    );
  });

  test('input environment', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Environment');
    await user.type(input, 'production');

    expect(input).toHaveValue('production');

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_ENVIRONMENT'
    );
  });

  test('input ref and type', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent();

    const input = screen.getByLabelText('Git Ref');
    await user.type(input, 'release-*');

    expect(input).toHaveValue('release-*');
    expect(screen.getByLabelText('Branch')).toHaveValue('release-*');

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_REF'
    );

    const select = screen.getByLabelText('Ref Type');
    await selectEvent.select(select, ['Tag']);

    expect(input).toHaveValue('release-*');
    expect(screen.getByLabelText('Branch')).toHaveValue('');

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_REF'
    );
  });

  test('input slug disabled', async () => {
    renderComponent();

    const input = screen.getByLabelText('Enterprise slug');
    expect(input).toBeDisabled();
  });

  test('input slug', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent({
      isEnterprise: true,
    });

    const input = screen.getByLabelText('Enterprise slug');
    await user.type(input, 'octo-enterprise');

    expect(input).toHaveValue('octo-enterprise');

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_ENTERPRISE_SLUG'
    );
  });

  test('input jwks disabled', async () => {
    renderComponent();

    const input = screen.getByLabelText('Enterprise JWKS');
    expect(input).toBeDisabled();
  });

  test('input jwks', async () => {
    const tracking = trackingTester();

    const { user } = renderComponent({
      isEnterprise: true,
      initialState: {
        gitHubUrl: 'example.com/owner/repo',
        info: {
          host: 'example.com',
          owner: 'owner',
          repository: 'repo',
        },
      },
    });

    const input = screen.getByLabelText('Enterprise JWKS');
    await user.type(input, '{{"keys":[[]}'); // Note escaping of "{" and "["

    expect(input).toHaveValue('{"keys":[]}');

    // Skip start event
    tracking.skip();

    // Field tracking is debounced, so move time along to send the event
    await act(() => jest.advanceTimersByTimeAsync(1000));
    tracking.assertField(
      expect.any(String),
      'INTEGRATION_ENROLL_STEP_MWIGHAK8S_CONNECT_GITHUB',
      'INTEGRATION_ENROLL_FIELD_MWIGHAK8S_GITHUB_ENTERPRISE_STATIC_JWKS'
    );
  });
});

function renderComponent(opts?: {
  initialState?: ComponentProps<typeof GitHubK8sFlowProvider>['intitialState'];
  isEnterprise?: boolean;
  disableTracking?: boolean;
}) {
  const user = userEvent.setup({
    advanceTimers: t => jest.advanceTimersByTime(t),
  });
  const onNextStep = jest.fn();
  const onPrevStep = jest.fn();
  return {
    ...render(<ConnectGitHub nextStep={onNextStep} prevStep={onPrevStep} />, {
      wrapper: makeWrapper(opts),
    }),
    user,
    onNextStep,
    onPrevStep,
  };
}

function makeWrapper(opts?: {
  initialState?: ComponentProps<typeof GitHubK8sFlowProvider>['intitialState'];
  isEnterprise?: boolean;
  disableTracking?: boolean;
}) {
  const { initialState, isEnterprise = false, disableTracking } = opts ?? {};

  cfg.isEnterprise = isEnterprise;
  cfg.edition = isEnterprise ? 'ent' : 'oss';

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

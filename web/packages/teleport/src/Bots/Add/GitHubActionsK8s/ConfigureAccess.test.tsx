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
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';

import { ConfigureAccess } from './ConfigureAccess';
import { GitHubK8sFlowProvider, useGitHubK8sFlow } from './useGitHubK8sFlow';

const server = setupServer();

beforeAll(() => {
  server.listen();

  // Basic mock for all tests
  server.use(genWizardCiCdSuccess());
});

afterAll(() => server.close());

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
        kubernetesUsers: ['user@example.com'],
      },
    });

    expect(
      screen.getByRole('heading', { name: 'Configure Access' })
    ).toBeInTheDocument();

    expect(screen.getByLabelText('Kubernetes Groups')).toBeInTheDocument();
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
    const { user } = renderComponent();

    const input = screen.getByLabelText('Kubernetes Groups');
    await selectEvent.create(input, 'system:masters');
    await user.type(input, 'viewers{enter}');

    expect(screen.getByText('system:masters')).toBeInTheDocument();
    expect(screen.getByText('viewers')).toBeInTheDocument();
  });

  test('input users', async () => {
    const { user } = renderComponent();

    const input = screen.getByLabelText('Kubernetes Users');
    await selectEvent.create(input, 'user1@example.com');
    await user.type(input, 'user2@example.com{enter}');

    expect(screen.getByText('user1@example.com')).toBeInTheDocument();
    expect(screen.getByText('user2@example.com')).toBeInTheDocument();
  });
});

function renderComponent(opts?: {
  initialState?: ReturnType<typeof useGitHubK8sFlow>['state'];
}) {
  const user = userEvent.setup();
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
}) {
  const { initialState } = opts ?? {};
  const ctx = createTeleportContext();

  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ContextProvider ctx={ctx}>
          <ConfiguredThemeProvider theme={darkTheme}>
            <GitHubK8sFlowProvider intitialState={initialState}>
              {children}
            </GitHubK8sFlowProvider>
          </ConfiguredThemeProvider>
        </ContextProvider>
      </QueryClientProvider>
    );
  };
}

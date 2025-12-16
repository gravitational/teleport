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
import { ComponentProps, PropsWithChildren } from 'react';
import selectEvent from 'react-select-event';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';

import { ConnectGitHub } from './ConnectGitHub';
import { GitHubK8sFlowProvider } from './useGitHubK8sFlow';

const server = setupServer();

beforeAll(() => {
  server.listen();

  // Basic mock for all tests
  server.use(genWizardCiCdSuccess());
});

afterAll(() => server.close());

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
        ref: 'main',
        refType: 'branch',
        workflow: 'my-workflow',
        kubernetesGroups: [],
        kubernetesLabels: [{ name: '*', values: ['*'] }],
        kubernetesUsers: [],
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

    expect(screen.getByPlaceholderText('ref/heads/main')).toHaveValue('main');

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

  test('input github url', async () => {
    const { user } = renderComponent();

    const input = screen.getByLabelText('Repository URL');
    await user.type(input, 'https://github.com/foo/bar');

    expect(input).toHaveValue('https://github.com/foo/bar');
  });

  test('input branch', async () => {
    const { user } = renderComponent();

    const input = screen.getByLabelText('Branch');
    await user.type(input, 'main');

    expect(input).toHaveValue('main');
    expect(screen.getByLabelText('Git Ref')).toHaveValue('main');
  });

  test('toggle allow any branch', async () => {
    const { user } = renderComponent();

    const input = screen.getByLabelText('Branch');
    await user.type(input, 'main');

    const check = screen.getByLabelText('Allow any branch');
    await user.click(check);

    expect(input).toHaveValue('');
  });

  test('input workflow', async () => {
    const { user } = renderComponent();

    const input = screen.getByLabelText('Workflow');
    await user.type(input, 'my-workflow');

    expect(input).toHaveValue('my-workflow');
  });

  test('input environment', async () => {
    const { user } = renderComponent();

    const input = screen.getByLabelText('Environment');
    await user.type(input, 'production');

    expect(input).toHaveValue('production');
  });

  test('input ref and type', async () => {
    const { user } = renderComponent();

    const input = screen.getByLabelText('Git Ref');
    await user.type(input, 'release-*');

    expect(input).toHaveValue('release-*');
    expect(screen.getByLabelText('Branch')).toHaveValue('release-*');

    const select = screen.getByLabelText('Ref Type');
    await selectEvent.select(select, ['Tag']);

    expect(input).toHaveValue('release-*');
    expect(screen.getByLabelText('Branch')).toHaveValue('');
  });

  test('input slug disabled', async () => {
    renderComponent();

    const input = screen.getByLabelText('Enterprise slug');
    expect(input).toBeDisabled();
  });

  test('input slug', async () => {
    const { user } = renderComponent({
      isEnterprise: true,
    });

    const input = screen.getByLabelText('Enterprise slug');
    await user.type(input, 'octo-enterprise');

    expect(input).toHaveValue('octo-enterprise');
  });

  test('input jwks disabled', async () => {
    renderComponent();

    const input = screen.getByLabelText('Enterprise JWKS');
    expect(input).toBeDisabled();
  });

  test('input jwks', async () => {
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
  });
});

function renderComponent(opts?: {
  initialState?: ComponentProps<typeof GitHubK8sFlowProvider>['intitialState'];
  isEnterprise?: boolean;
}) {
  const user = userEvent.setup();
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
}) {
  const { initialState, isEnterprise = false } = opts ?? {};

  cfg.isEnterprise = isEnterprise;
  cfg.edition = isEnterprise ? 'ent' : 'oss';

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

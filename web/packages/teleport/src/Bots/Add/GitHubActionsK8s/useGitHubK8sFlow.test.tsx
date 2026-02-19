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
import { act, renderHook, waitFor } from '@testing-library/react';
import { PropsWithChildren } from 'react';

import { enableMswServer, server, testQueryClient } from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { TrackingProvider } from '../Shared/useTracking';
import { GitHubK8sFlowProvider, useGitHubK8sFlow } from './useGitHubK8sFlow';

enableMswServer();

beforeEach(() => {
  server.use(userEventCaptureSuccess());

  // The templates API call is debounced, so we'll need to time travel a little.
  jest.useFakeTimers();
});

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.useRealTimers();
  jest.clearAllMocks();
});

describe('useGitHubK8sFlow', () => {
  test('initial', async () => {
    withGenWizardCiCdSuccess({
      response: {
        terraform: 'mock terraform template',
      },
    });

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    await waitForGenerateCall(result);

    expect(result.current.state).toStrictEqual(withDefaultState({}));
    expect(result.current.template.terraform.data).toBe(
      'mock terraform template'
    );

    expect(result.current.template.ghaWorkflow).toContain(
      'TELEPORT_PROXY_ADDR: "some-long-cluster-public-url-name.cloud.teleport.gravitational.io:1234"'
    );
  });

  describe('github url', () => {
    test.each`
      name                | url
      ${'with scheme'}    | ${'https://github.example.com/owner/repo'}
      ${'without scheme'} | ${'github.example.com/owner/repo'}
    `('$name', async ({ url }) => {
      withGenWizardCiCdSuccess();

      const { result } = renderHook(() => useGitHubK8sFlow(), {
        wrapper: Wrapper,
      });

      act(() => {
        result.current.dispatch({
          type: 'github-url-changed',
          value: url,
        });
      });

      expect(result.current.state).toStrictEqual(
        withDefaultState({
          gitHubUrl: url,
          info: {
            host: 'github.example.com',
            owner: 'owner',
            repository: 'repo',
          },
        })
      );

      await waitForGenerateCall(result);

      expect(result.current.template.terraform.data).toContain(
        '"enterprise_server_host":"github.example.com"'
      );
      expect(result.current.template.terraform.data).toContain(
        '"repository":"repo"'
      );
      expect(result.current.template.terraform.data).toContain(
        '"owner":"owner"'
      );

      expect(result.current.template.ghaWorkflow).toContain(
        'TELEPORT_JOIN_TOKEN_NAME: "gha-owner-repo"'
      );
    });
  });

  test('branch', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'ref-changed',
        value: 'release-*',
      });

      result.current.dispatch({
        type: 'allow-any-branch-toggled',
        value: true,
      });

      result.current.dispatch({
        type: 'branch-changed',
        value: 'release-*',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        branch: 'release-*',
        ref: 'refs/heads/release-*',
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"ref":"refs/heads/release-*"'
    );
    expect(result.current.template.terraform.data).toContain(
      '"ref_type":"branch"'
    );
  });

  test('allow any branch', async () => {
    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'allow-any-branch-toggled',
        value: true,
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        allowAnyBranch: true,
      })
    );
  });

  test('workflow', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'workflow-changed',
        value: 'my-workflow',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        workflow: 'my-workflow',
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"workflow":"my-workflow"'
    );
  });

  test('environment', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'environment-changed',
        value: 'production',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        environment: 'production',
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"environment":"production"'
    );
  });

  test('ref', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'ref-changed',
        value: 'release-*',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        ref: 'release-*',
        branch: 'release-*',
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"ref":"release-*"'
    );
  });

  test('ref type', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'ref-changed',
        value: 'release-*',
      });

      result.current.dispatch({
        type: 'ref-type-changed',
        value: 'tag',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        refType: 'tag',
        ref: 'release-*',
        branch: '',
        isBranchDisabled: true,
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"ref_type":"tag"'
    );
  });

  test('slug', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'slug-changed',
        value: 'octo-enterprise',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        enterpriseSlug: 'octo-enterprise',
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"enterprise_slug":"octo-enterprise"'
    );
  });

  test('jwks', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'jwks-changed',
        value: '{"keys": []}',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        enterpriseJwks: '{"keys": []}',
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"static_jwks":"{\\"keys\\": []}"'
    );
  });

  test('kubernetes cluster', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'kubernetes-cluster-changed',
        value: 'my-kubernetes-cluster',
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        kubernetesCluster: 'my-kubernetes-cluster',
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.ghaWorkflow).toContain(
      'TELEPORT_K8S_CLUSTER_NAME: "my-kubernetes-cluster"'
    );
  });

  test('kubernetes groups', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'kubernetes-groups-changed',
        value: ['system:masters'],
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        kubernetesGroups: ['system:masters'],
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"groups":["system:masters"]'
    );
  });

  test('kubernetes labels', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'kubernetes-labels-changed',
        value: [{ name: 'foo', values: ['bar'] }],
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        kubernetesLabels: [
          {
            name: 'foo',
            values: ['bar'],
          },
        ],
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"labels":{"foo":["bar"]}'
    );
  });

  test('kubernetes users', async () => {
    withGenWizardCiCdSuccess();

    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    act(() => {
      result.current.dispatch({
        type: 'kubernetes-users-changed',
        value: ['user1@example.com'],
      });
    });

    expect(result.current.state).toStrictEqual(
      withDefaultState({
        kubernetesUsers: ['user1@example.com'],
      })
    );

    await waitForGenerateCall(result);

    expect(result.current.template.terraform.data).toContain(
      '"users":["user1@example.com"]'
    );
  });
});

async function waitForGenerateCall(result: {
  current: ReturnType<typeof useGitHubK8sFlow>;
}) {
  await act(jest.advanceTimersToNextTimerAsync);
  return waitFor(() => {
    expect(result.current.template.terraform.loading).toBeFalsy();
  });
}

function withDefaultState(
  overrides: Partial<ReturnType<typeof useGitHubK8sFlow>['state']>
) {
  return {
    allowAnyBranch: false,
    branch: '',
    enterpriseJwks: '',
    enterpriseSlug: '',
    environment: '',
    gitHubUrl: '',
    isBranchDisabled: false,
    ref: '',
    refType: 'branch',
    workflow: '',
    kubernetesGroups: [],
    kubernetesLabels: [
      {
        name: '*',
        values: ['*'],
      },
    ],
    kubernetesUsers: [],
    kubernetesCluster: '',
    ...overrides,
  };
}

function withGenWizardCiCdSuccess(
  ...params: Parameters<typeof genWizardCiCdSuccess>
) {
  server.use(genWizardCiCdSuccess(...params));
}

function Wrapper(props: PropsWithChildren) {
  const ctx = createTeleportContext();
  return (
    <QueryClientProvider client={testQueryClient}>
      <ContextProvider ctx={ctx}>
        <TrackingProvider>
          <GitHubK8sFlowProvider>{props.children}</GitHubK8sFlowProvider>
        </TrackingProvider>
      </ContextProvider>
    </QueryClientProvider>
  );
}

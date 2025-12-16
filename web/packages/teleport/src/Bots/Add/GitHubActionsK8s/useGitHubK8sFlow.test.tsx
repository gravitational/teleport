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
import { act, renderHook } from '@testing-library/react';
import { PropsWithChildren } from 'react';

import { testQueryClient } from 'design/utils/testing';

import { GitHubK8sFlowProvider, useGitHubK8sFlow } from './useGitHubK8sFlow';

describe('useGitHubK8sFlow', () => {
  test('initial', async () => {
    const { result } = renderHook(() => useGitHubK8sFlow(), {
      wrapper: Wrapper,
    });

    expect(result.current.state).toStrictEqual(withDefaultState({}));
  });

  describe('github url', () => {
    test.each`
      name                | url
      ${'with scheme'}    | ${'https://github.com/gravitational/teleport'}
      ${'without scheme'} | ${'github.com/gravitational/teleport'}
    `('$name', async ({ url }) => {
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
            host: 'github.com',
            owner: 'gravitational',
            repository: 'teleport',
          },
        })
      );
    });
  });

  test('branch', async () => {
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
        ref: 'release-*',
      })
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
  });

  test('environment', async () => {
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
  });

  test('ref', async () => {
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
  });

  test('ref type', async () => {
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
  });

  test('slug', async () => {
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
  });

  test('jwks', async () => {
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
  });

  test('kubernetes groups', async () => {
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
  });

  test('kubernetes users', async () => {
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
  });
});

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
    kubernetesUsers: [],
    ...overrides,
  };
}

function Wrapper(props: PropsWithChildren) {
  return (
    <QueryClientProvider client={testQueryClient}>
      <GitHubK8sFlowProvider>{props.children}</GitHubK8sFlowProvider>
    </QueryClientProvider>
  );
}

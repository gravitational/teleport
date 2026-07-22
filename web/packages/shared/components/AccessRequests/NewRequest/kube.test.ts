/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Attempt } from 'shared/hooks/useAttemptNext';

import { checkSupportForKubeResources } from './kube';

describe('requestKubeResourceSupported()', () => {
  const testCases: {
    name: string;
    attempt: Attempt;
    expected: {
      requestKubeResourceSupported: boolean;
      isRequestKubeResourceError: boolean;
    };
  }[] = [
    {
      name: 'non failed status',
      attempt: { status: '' },
      expected: {
        requestKubeResourceSupported: true,
        isRequestKubeResourceError: false,
      },
    },
    {
      name: 'non request.kubernetes_resources related error',
      attempt: {
        status: 'failed',
        statusText: `some other error`,
      },
      expected: {
        requestKubeResourceSupported: true,
        isRequestKubeResourceError: false,
      },
    },
    {
      name: 'with supported namespace in error (single role)',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: test-role-1: [namespace]`,
      },
      expected: {
        requestKubeResourceSupported: true,
        isRequestKubeResourceError: true,
      },
    },
    {
      name: 'with supported namespace in error (multi role)',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: test-role-1: [pod secret], test-role-2: [deployment namespace]`,
      },
      expected: {
        requestKubeResourceSupported: true,
        isRequestKubeResourceError: true,
      },
    },
    {
      name: 'with supported kube_cluster in error (multi role)',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: test-role-1: [pod secret], test-role-2: [deployment kube_cluster]`,
      },
      expected: {
        requestKubeResourceSupported: true,
        isRequestKubeResourceError: true,
      },
    },
    {
      name: 'with supported kube_cluster and namespace in error (multi role)',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: test-role-1: [pod], test-role-2: [namespace kube_cluster]`,
      },
      expected: {
        requestKubeResourceSupported: true,
        isRequestKubeResourceError: true,
      },
    },
    {
      name: 'without supported kinds in error',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: test-role-1: [deployment], test-role-2: [pod secret]`,
      },
      expected: {
        requestKubeResourceSupported: false,
        isRequestKubeResourceError: true,
      },
    },
    // empty bracket case can happen from admin configuration error
    // where allow and deny canceled each other so nothing is allowed.
    {
      name: 'empty bracket with space',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: test-role-1: [ ]`,
      },
      expected: {
        requestKubeResourceSupported: false,
        isRequestKubeResourceError: true,
      },
    },
    {
      name: 'empty bracket without space',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: test-role-1: []`,
      },
      expected: {
        requestKubeResourceSupported: false,
        isRequestKubeResourceError: true,
      },
    },
    // should never happen but just in case
    {
      name: 'without any role',
      attempt: {
        status: 'failed',
        statusText: `your Teleport role's "request.kubernetes_resources" field did not allow requesting \
        to some or all of the requested Kubernetes resources. allowed kinds for \
        each requestable roles: `,
      },
      expected: {
        requestKubeResourceSupported: true,
        isRequestKubeResourceError: false,
      },
    },
  ];

  test.each(testCases)('$name', ({ attempt, expected }) => {
    expect(checkSupportForKubeResources(attempt)).toEqual(expected);
  });
});

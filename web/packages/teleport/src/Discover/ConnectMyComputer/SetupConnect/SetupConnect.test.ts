/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { renderHook } from '@testing-library/react-hooks';

import * as useTeleport from 'teleport/useTeleport';
import NodeService, { Node } from 'teleport/services/nodes';
import UserService from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';

import { nodes } from 'teleport/Nodes/fixtures';

import { usePollForConnectMyComputerNode } from './SetupConnect';

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('usePollForConnectMyComputerNode', () => {
  const tests: Array<{
    name: string;
    initialNodes: Node[];
  }> = [
    {
      name: 'returns the correct node if the first polling request returns no nodes',
      initialNodes: [],
    },
    {
      name: 'returns the correct node if the first polling request returns some nodes',
      initialNodes: [nodes[1], nodes[2]],
    },
  ];

  test.each(tests)('$name', async ({ initialNodes }) => {
    const expectedNode = nodes[0];

    const nodeService = {
      fetchNodes: jest.fn(),
    } as Partial<NodeService> as NodeService;

    jest
      .mocked(nodeService)
      .fetchNodes.mockResolvedValue({ agents: [...initialNodes, expectedNode] })
      .mockResolvedValueOnce({ agents: initialNodes });

    jest
      .spyOn(useTeleport, 'default')
      .mockReturnValue({ nodeService } as TeleportContext);

    const { result, waitForValueToChange } = renderHook(() =>
      usePollForConnectMyComputerNode({
        username: 'alice',
        clusterId: 'foo',
        pingInterval: 1,
        reloadUser: false,
      })
    );

    expect(result.error).toBeUndefined();
    expect(result.current.node).toBeFalsy();
    expect(result.current.isPolling).toBe(true);

    await waitForValueToChange(() => result.current.node, { interval: 3 });

    expect(result.current.node).toEqual(expectedNode);
    expect(result.current.isPolling).toBe(false);
  });

  it('reloads user before each poll if reloadUser is true', async () => {
    const expectedNode = nodes[0];
    let hasReloadedUser = false;

    const nodeService = {
      fetchNodes: jest.fn(),
    } as Partial<NodeService> as NodeService;

    jest.mocked(nodeService).fetchNodes.mockImplementation(async () => {
      if (hasReloadedUser) {
        return { agents: [expectedNode] };
      } else {
        return { agents: [] };
      }
    });

    const userService = {
      reloadUser: jest.fn(),
    } as Partial<typeof UserService> as typeof UserService;

    jest.mocked(userService).reloadUser.mockImplementation(async () => {
      hasReloadedUser = true;
    });

    jest
      .spyOn(useTeleport, 'default')
      .mockReturnValue({ nodeService, userService } as TeleportContext);

    const { result, rerender, waitFor, waitForValueToChange } = renderHook(
      usePollForConnectMyComputerNode,
      {
        initialProps: {
          reloadUser: false,
          username: 'alice',
          clusterId: 'foo',
          pingInterval: 1,
        },
      }
    );
    expect(result.error).toBeUndefined();
    await waitFor(() => {
      expect(nodeService.fetchNodes).toHaveBeenCalled();
    });
    expect(userService.reloadUser).not.toHaveBeenCalled();

    rerender({
      reloadUser: true,
      username: 'alice',
      clusterId: 'foo',
      pingInterval: 1,
    });
    expect(result.error).toBeUndefined();

    await waitForValueToChange(() => result.current.node, { interval: 3 });
    expect(userService.reloadUser).toHaveBeenCalled();

    expect(result.current.node).toEqual(expectedNode);
    expect(result.current.isPolling).toBe(false);
  });
});

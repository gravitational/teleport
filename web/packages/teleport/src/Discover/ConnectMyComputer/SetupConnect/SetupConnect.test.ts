/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { renderHook, waitFor } from '@testing-library/react';

import { nodes } from 'teleport/Nodes/fixtures';
import NodeService, { Node } from 'teleport/services/nodes';
import UserService from 'teleport/services/user';
import TeleportContext from 'teleport/teleportContext';
import * as useTeleport from 'teleport/useTeleport';

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

    const { result } = renderHook(() =>
      usePollForConnectMyComputerNode({
        username: 'alice',
        clusterId: 'foo',
        pingInterval: 1,
        reloadUser: false,
      })
    );

    expect(result.current.node).toBeFalsy();
    expect(result.current.isPolling).toBe(true);

    await waitFor(() => expect(result.current.node).toEqual(expectedNode), {
      interval: 3,
    });
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

    const { result, rerender } = renderHook(usePollForConnectMyComputerNode, {
      initialProps: {
        reloadUser: false,
        username: 'alice',
        clusterId: 'foo',
        pingInterval: 1,
      },
    });
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

    await waitFor(() => expect(result.current.node).toBeTruthy(), {
      interval: 3,
    });

    expect(userService.reloadUser).toHaveBeenCalled();

    expect(result.current.node).toEqual(expectedNode);
    expect(result.current.isPolling).toBe(false);
  });
});

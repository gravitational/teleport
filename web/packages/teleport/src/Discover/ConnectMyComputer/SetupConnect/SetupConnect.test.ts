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
import NodeService from 'teleport/services/nodes/nodes';
import TeleportContext from 'teleport/teleportContext';

import { nodes } from 'teleport/Nodes/fixtures';

import { Node } from 'teleport/services/nodes';

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
      })
    );

    expect(result.error).toBeUndefined();
    expect(result.current.node).toBeFalsy();
    expect(result.current.isPolling).toBe(true);

    await waitForValueToChange(() => result.current.node, { interval: 3 });

    expect(result.current.node).toEqual(expectedNode);
    expect(result.current.isPolling).toBe(false);
  });
});

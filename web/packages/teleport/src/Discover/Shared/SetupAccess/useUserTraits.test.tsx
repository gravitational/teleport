/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { MemoryRouter } from 'react-router';
import { renderHook, act } from '@testing-library/react-hooks';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { DiscoverProvider } from 'teleport/Discover/useDiscover';
import cfg from 'teleport/config';
import { userEventService } from 'teleport/services/userEvent';

import { ResourceKind } from '../ResourceKind';

import { useUserTraits } from './useUserTraits';

import type {
  AgentMeta,
  DbMeta,
  KubeMeta,
  NodeMeta,
} from 'teleport/Discover/useDiscover';

describe('onProceed correctly deduplicates, removes static traits, updates meta, and calls updateUser', () => {
  const ctx = createTeleportContext();
  jest.spyOn(ctx.userService, 'fetchUser').mockResolvedValue(getMockUser());
  jest.spyOn(ctx.userService, 'updateUser').mockResolvedValue(null);
  jest.spyOn(ctx.userService, 'applyUserTraits').mockResolvedValue(null);
  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(null as never); // return value does not matter but required by ts

  let wrapper;

  beforeEach(() => {
    wrapper = ({ children }) => (
      <MemoryRouter initialEntries={[{ pathname: cfg.routes.discover }]}>
        <ContextProvider ctx={ctx}>
          <FeaturesContextProvider value={[]}>
            <DiscoverProvider>{children}</DiscoverProvider>
          </FeaturesContextProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('kubernetes', async () => {
    const props = {
      agentMeta: getMeta(ResourceKind.Kubernetes) as AgentMeta,
      updateAgentMeta: jest.fn(x => x),
      nextStep: () => null,
      resourceSpec: { kind: ResourceKind.Kubernetes } as any,
    };

    const { result, waitForNextUpdate, waitFor } = renderHook(
      () => useUserTraits(props),
      {
        wrapper,
      }
    );
    await waitForNextUpdate();

    const staticTraits = result.current.staticTraits;
    const dynamicTraits = result.current.dynamicTraits;

    const mockedSelectedOptions = {
      kubeUsers: [
        {
          isFixed: true,
          label: staticTraits.kubeUsers[0],
          value: staticTraits.kubeUsers[0],
        },
        {
          isFixed: false,
          label: dynamicTraits.kubeUsers[0],
          value: dynamicTraits.kubeUsers[0],
        },
      ],
      kubeGroups: [
        // duplicates
        {
          isFixed: false,
          label: dynamicTraits.kubeGroups[0],
          value: dynamicTraits.kubeGroups[0],
        },
        {
          isFixed: false,
          label: dynamicTraits.kubeGroups[0],
          value: dynamicTraits.kubeGroups[0],
        },
      ],
    };

    const expected = {
      kubeUsers: [dynamicTraits.kubeUsers[0]],
      kubeGroups: [dynamicTraits.kubeGroups[0]],
    };

    act(() => {
      result.current.onProceed(mockedSelectedOptions);
    });

    await waitFor(() => {
      expect(ctx.userService.applyUserTraits).toHaveBeenCalledTimes(1);
    });

    // Test that we are updating the user with the correct traits.
    const mockUser = getMockUser();
    expect(ctx.userService.updateUser).toHaveBeenCalledWith({
      ...mockUser,
      traits: { ...mockUser.traits, ...expected },
    });

    // Test that updating meta correctly updated the dynamic traits.
    const updatedMeta = props.updateAgentMeta.mock.results[0].value as KubeMeta;
    expect(updatedMeta.kube.users).toStrictEqual([
      ...staticTraits.kubeUsers,
      ...expected.kubeUsers,
    ]);
    expect(updatedMeta.kube.groups).toStrictEqual([
      ...staticTraits.kubeGroups,
      ...expected.kubeGroups,
    ]);
  });

  test('database', async () => {
    const props = {
      agentMeta: getMeta(ResourceKind.Database) as AgentMeta,
      updateAgentMeta: jest.fn(x => x),
      nextStep: () => null,
      resourceSpec: { kind: ResourceKind.Database } as any,
    };

    const { result, waitForNextUpdate, waitFor } = renderHook(
      () => useUserTraits(props),
      {
        wrapper,
      }
    );
    await waitForNextUpdate();

    const staticTraits = result.current.staticTraits;
    const dynamicTraits = result.current.dynamicTraits;

    const mockedSelectedOptions = {
      databaseNames: [
        {
          isFixed: true,
          label: staticTraits.databaseNames[0],
          value: staticTraits.databaseNames[0],
        },
        {
          isFixed: false,
          label: dynamicTraits.databaseNames[0],
          value: dynamicTraits.databaseNames[0],
        },
      ],
      databaseUsers: [
        // duplicates
        {
          isFixed: false,
          label: dynamicTraits.databaseUsers[0],
          value: dynamicTraits.databaseUsers[0],
        },
        {
          isFixed: false,
          label: dynamicTraits.databaseUsers[0],
          value: dynamicTraits.databaseUsers[0],
        },
      ],
    };

    const expected = {
      databaseNames: [dynamicTraits.databaseNames[0]],
      databaseUsers: [dynamicTraits.databaseUsers[0]],
    };

    act(() => {
      result.current.onProceed(mockedSelectedOptions);
    });

    await waitFor(() => {
      expect(ctx.userService.applyUserTraits).toHaveBeenCalledTimes(1);
    });

    // Test that we are updating the user with the correct traits.
    const mockUser = getMockUser();
    expect(ctx.userService.updateUser).toHaveBeenCalledWith({
      ...mockUser,
      traits: { ...mockUser.traits, ...expected },
    });

    // Test that updating meta correctly updated the dynamic traits.
    const updatedMeta = props.updateAgentMeta.mock.results[0].value as DbMeta;
    expect(updatedMeta.db.users).toStrictEqual([
      ...staticTraits.databaseUsers,
      ...expected.databaseUsers,
    ]);
    expect(updatedMeta.db.names).toStrictEqual([
      ...staticTraits.databaseNames,
      ...expected.databaseNames,
    ]);
  });

  test('node', async () => {
    const props = {
      agentMeta: getMeta(ResourceKind.Server) as AgentMeta,
      updateAgentMeta: jest.fn(x => x),
      nextStep: () => null,
      resourceSpec: { kind: ResourceKind.Server } as any,
    };

    const { result, waitForNextUpdate, waitFor } = renderHook(
      () => useUserTraits(props),
      {
        wrapper,
      }
    );
    await waitForNextUpdate();

    const staticTraits = result.current.staticTraits;
    const dynamicTraits = result.current.dynamicTraits;

    const mockedSelectedOptions = {
      logins: [
        {
          isFixed: true,
          label: staticTraits.logins[0],
          value: staticTraits.logins[0],
        },
        {
          isFixed: false,
          label: dynamicTraits.logins[0],
          value: dynamicTraits.logins[0],
        },
        // duplicate
        {
          isFixed: false,
          label: dynamicTraits.logins[0],
          value: dynamicTraits.logins[0],
        },
      ],
    };

    const expected = {
      logins: [dynamicTraits.logins[0]],
    };

    act(() => {
      result.current.onProceed(mockedSelectedOptions);
    });

    await waitFor(() => {
      expect(ctx.userService.applyUserTraits).toHaveBeenCalledTimes(1);
    });

    // Test that we are updating the user with the correct traits.
    const mockUser = getMockUser();
    expect(ctx.userService.updateUser).toHaveBeenCalledWith({
      ...mockUser,
      traits: { ...mockUser.traits, ...expected },
    });

    // Test that updating meta correctly updated the dynamic traits.
    const updatedMeta = props.updateAgentMeta.mock.results[0].value as NodeMeta;
    expect(updatedMeta.node.sshLogins).toStrictEqual([
      ...staticTraits.logins,
      ...expected.logins,
    ]);
  });
});

describe('static and dynamic traits are correctly separated and correctly creates Option objects', () => {
  test.each`
    resourceKind               | traitName
    ${ResourceKind.Kubernetes} | ${'kubeUsers'}
    ${ResourceKind.Kubernetes} | ${'kubeGroups'}
    ${ResourceKind.Server}     | ${'logins'}
    ${ResourceKind.Database}   | ${'databaseNames'}
    ${ResourceKind.Database}   | ${'databaseUsers'}
  `('$traitName', async ({ resourceKind, traitName }) => {
    const ctx = createTeleportContext();
    jest.spyOn(ctx.userService, 'fetchUser').mockResolvedValue(getMockUser());

    const props = {
      agentMeta: getMeta(resourceKind) as AgentMeta,
      updateAgentMeta: () => null,
      nextStep: () => null,
      resourceSpec: { kind: resourceKind } as any,
    };

    const wrapper = ({ children }) => (
      <MemoryRouter initialEntries={[{ pathname: cfg.routes.discover }]}>
        <ContextProvider ctx={ctx}>
          <FeaturesContextProvider value={[]}>
            <DiscoverProvider>{children}</DiscoverProvider>
          </FeaturesContextProvider>
        </ContextProvider>
      </MemoryRouter>
    );

    const { result, waitForNextUpdate } = renderHook(
      () => useUserTraits(props),
      {
        wrapper,
      }
    );

    await waitForNextUpdate();
    expect(ctx.userService.fetchUser).toHaveBeenCalled();

    // Test correct making of dynamic traits.
    const dynamicTraits = result.current.dynamicTraits;
    const dynamicOptions = [
      {
        isFixed: false,
        label: dynamicTraits[traitName][0],
        value: dynamicTraits[traitName][0],
      },
      {
        isFixed: false,
        label: dynamicTraits[traitName][1],
        value: dynamicTraits[traitName][1],
      },
    ];
    expect(result.current.getSelectableOptions(traitName)).toStrictEqual(
      dynamicOptions
    );

    // Test correct making of static traits.
    const staticTraits = result.current.staticTraits;
    const staticOptions = [
      {
        isFixed: true,
        label: staticTraits[traitName][0],
        value: staticTraits[traitName][0],
      },
      {
        isFixed: true,
        label: staticTraits[traitName][1],
        value: staticTraits[traitName][1],
      },
    ];
    expect(result.current.getFixedOptions(traitName)).toStrictEqual(
      staticOptions
    );

    // Test correct making of both static and dynamic traits.
    expect(result.current.initSelectedOptions(traitName)).toStrictEqual([
      ...staticOptions,
      ...dynamicOptions,
    ]);
  });
});

function getMockUser() {
  return {
    name: 'llama',
    roles: [],
    authType: '',
    isLocal: true,
    traits: {
      logins: ['dynamicLogin1', 'dynamicLogin2'],
      databaseUsers: ['dynamicDbUser1', 'dynamicDbUser2'],
      databaseNames: ['dynamicDbName1', 'dynamicDbName2'],
      kubeUsers: ['dynamicKbUser1', 'dynamicKbUser2'],
      kubeGroups: ['dynamicKbGroup1', 'dynamicKbGroup1'],
      windowsLogins: [],
      awsRoleArns: [],
    },
  };
}

function getMeta(resource: ResourceKind) {
  const mockUser = getMockUser();
  switch (resource) {
    case ResourceKind.Kubernetes:
      return {
        resourceName: '',
        kube: {
          name: '',
          labels: [],
          users: [
            ...mockUser.traits.kubeUsers,
            'staticKbUser1',
            'staticKbUser2',
          ],
          groups: [
            ...mockUser.traits.kubeGroups,
            'staticKbGroup1',
            'staticKbGroup2',
          ],
        },
      } as KubeMeta;
    case ResourceKind.Server:
      return {
        resourceName: '',
        node: {
          id: '',
          clusterId: '',
          hostname: '',
          labels: [],
          addr: '',
          tunnel: false,
          sshLogins: [
            ...mockUser.traits.logins,
            'staticLogin1',
            'staticLogin2',
          ],
        },
      } as NodeMeta;
    case ResourceKind.Database:
      return {
        resourceName: '',
        db: {
          name: '',
          description: '',
          type: '',
          protocol: 'postgres',
          labels: [],
          names: [
            ...mockUser.traits.databaseNames,
            'staticDbName1',
            'staticDbName2',
          ],
          users: [
            ...mockUser.traits.databaseUsers,
            'staticDbUser1',
            'staticDbUser2',
          ],
        },
      } as DbMeta;
  }
}

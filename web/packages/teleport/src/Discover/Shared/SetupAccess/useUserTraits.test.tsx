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
import {
  defaultDiscoverContext,
  defaultResourceSpec,
} from 'teleport/Discover/Fixtures/fixtures';
import TeleportContext from 'teleport/teleportContext';
import { ExcludeUserField } from 'teleport/services/user';

import { ResourceKind } from '../ResourceKind';

import { useUserTraits } from './useUserTraits';

import type {
  DbMeta,
  DiscoverContextState,
  KubeMeta,
  NodeMeta,
} from 'teleport/Discover/useDiscover';

describe('onProceed correctly deduplicates, removes static traits, updates meta, and calls updateUser', () => {
  const teleCtx = createTeleportContext();
  jest.spyOn(teleCtx.userService, 'fetchUser').mockResolvedValue(getMockUser());
  jest.spyOn(teleCtx.userService, 'updateUser').mockResolvedValue(null);
  jest.spyOn(teleCtx.userService, 'reloadUser').mockResolvedValue(null);
  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(null as never); // return value does not matter but required by ts

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('kubernetes', async () => {
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Kubernetes),
    });
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Kubernetes),
    };
    const spyUpdateAgentMeta = jest
      .spyOn(discoverCtx, 'updateAgentMeta')
      .mockImplementation(x => x);

    const { result, waitFor } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.staticTraits.kubeUsers).toHaveLength(2)
    );

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

    await waitFor(() => {
      expect(teleCtx.userService.fetchUser).toHaveBeenCalledTimes(1);
    });

    act(() => {
      result.current.onProceed(mockedSelectedOptions);
    });

    await waitFor(() => {
      expect(teleCtx.userService.reloadUser).toHaveBeenCalledTimes(1);
    });

    // Test that we are updating the user with the correct traits.
    const mockUser = getMockUser();
    expect(teleCtx.userService.updateUser).toHaveBeenCalledWith(
      {
        ...mockUser,
        traits: { ...mockUser.traits, ...expected },
      },
      ExcludeUserField.AllTraits
    );

    // Test that updating meta correctly updated the dynamic traits.
    const updatedMeta = spyUpdateAgentMeta.mock.results[0].value as KubeMeta;
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
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Database),
    });
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Database),
    };
    const spyUpdateAgentMeta = jest
      .spyOn(discoverCtx, 'updateAgentMeta')
      .mockImplementation(x => x);

    const { result, waitFor } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.staticTraits.databaseNames).toHaveLength(2)
    );

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
      expect(teleCtx.userService.reloadUser).toHaveBeenCalledTimes(1);
    });

    // Test that we are updating the user with the correct traits.
    const mockUser = getMockUser();
    expect(teleCtx.userService.updateUser).toHaveBeenCalledWith(
      {
        ...mockUser,
        traits: { ...mockUser.traits, ...expected },
      },
      ExcludeUserField.AllTraits
    );

    // Test that updating meta correctly updated the dynamic traits.
    const updatedMeta = spyUpdateAgentMeta.mock.results[0].value as DbMeta;
    expect(updatedMeta.db.users).toStrictEqual([
      ...staticTraits.databaseUsers,
      ...expected.databaseUsers,
    ]);
    expect(updatedMeta.db.names).toStrictEqual([
      ...staticTraits.databaseNames,
      ...expected.databaseNames,
    ]);
  });

  test('database with auto discover preserves existing + new dynamic traits', async () => {
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Database),
    });
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Database),
      autoDiscovery: {
        config: { name: '', discoveryGroup: '', aws: [] },
        requiredVpcsAndSubnets: {},
      },
    };

    const { result, waitFor } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.dynamicTraits.databaseNames).toHaveLength(2)
    );

    expect(result.current.dynamicTraits.databaseUsers).toHaveLength(2);

    // Should not be setting statics.
    expect(result.current.staticTraits.databaseNames).toHaveLength(0);
    expect(result.current.staticTraits.databaseUsers).toHaveLength(0);

    const addedTraitsOpts = {
      databaseNames: [
        {
          isFixed: true,
          label: 'banana',
          value: 'banana',
        },
        {
          isFixed: true,
          label: 'carrot',
          value: 'carrot',
        },
      ],
      databaseUsers: [
        {
          isFixed: false,
          label: 'apple',
          value: 'apple',
        },
      ],
    };

    act(() => {
      result.current.onProceed(addedTraitsOpts);
    });

    await waitFor(() => {
      expect(teleCtx.userService.reloadUser).toHaveBeenCalledTimes(1);
    });

    // Test that we are updating the user with the correct traits.
    const mockUser = getMockUser();
    expect(teleCtx.userService.updateUser).toHaveBeenCalledWith(
      {
        ...mockUser,
        traits: {
          ...result.current.dynamicTraits,
          databaseNames: ['banana', 'carrot'],
          databaseUsers: ['apple'],
        },
      },
      ExcludeUserField.AllTraits
    );
  });

  test('node', async () => {
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Server),
    });
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Server),
    };
    const spyUpdateAgentMeta = jest
      .spyOn(discoverCtx, 'updateAgentMeta')
      .mockImplementation(x => x);

    const { result, waitFor } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.staticTraits.logins).toHaveLength(2)
    );

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
      expect(teleCtx.userService.reloadUser).toHaveBeenCalledTimes(1);
    });

    // Test that we are updating the user with the correct traits.
    const mockUser = getMockUser();
    expect(teleCtx.userService.updateUser).toHaveBeenCalledWith(
      {
        ...mockUser,
        traits: { ...mockUser.traits, ...expected },
      },
      ExcludeUserField.AllTraits
    );

    // Test that updating meta correctly updated the dynamic traits.
    const updatedMeta = spyUpdateAgentMeta.mock.results[0].value as NodeMeta;
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
    const teleCtx = createTeleportContext();
    jest
      .spyOn(teleCtx.userService, 'fetchUser')
      .mockResolvedValue(getMockUser());

    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(resourceKind),
    });
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(resourceKind),
    };

    const { result, waitForNextUpdate } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitForNextUpdate();
    expect(teleCtx.userService.fetchUser).toHaveBeenCalled();

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

describe('calls to nextStep respects number of steps to skip', () => {
  test('with auto discover, as a sso user with no traits', async () => {
    const teleCtx = createTeleportContext();
    teleCtx.storeUser.state.authType = 'sso';
    const user = getMockUser();
    user.traits = {
      logins: ['login'],
      databaseUsers: [],
      databaseNames: [],
      kubeUsers: [],
      kubeGroups: [],
      windowsLogins: [],
      awsRoleArns: [],
    };

    jest.spyOn(teleCtx.userService, 'fetchUser').mockResolvedValue(user);

    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Database),
    });
    discoverCtx.agentMeta.autoDiscovery = {
      config: { name: '', discoveryGroup: '', aws: [] },
      requiredVpcsAndSubnets: {},
    };

    const { result, waitForNextUpdate } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitForNextUpdate();
    expect(result.current.dynamicTraits.logins.length).toBeGreaterThan(0);

    act(() => {
      result.current.onProceed({ databaseNames: [], databaseUsers: [] }, 7);
    });

    expect(discoverCtx.nextStep).toHaveBeenCalledWith(7);
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

function wrapperFn(
  discoverCtx: DiscoverContextState,
  teleportCtx: TeleportContext
) {
  return ({ children }) => (
    <MemoryRouter initialEntries={[{ pathname: cfg.routes.discover }]}>
      <ContextProvider ctx={teleportCtx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>{children}</DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

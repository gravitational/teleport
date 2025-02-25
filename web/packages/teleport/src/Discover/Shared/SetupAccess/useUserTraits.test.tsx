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

import { act, renderHook, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { AwsRole } from 'shared/services/apps';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { app } from 'teleport/Discover/AwsMangementConsole/fixtures';
import {
  defaultDiscoverContext,
  defaultResourceSpec,
} from 'teleport/Discover/Fixtures/fixtures';
import {
  DiscoverProvider,
  type AppMeta,
  type DbMeta,
  type DiscoverContextState,
  type KubeMeta,
  type NodeMeta,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ExcludeUserField } from 'teleport/services/user';
import { userEventService } from 'teleport/services/userEvent';
import TeleportContext from 'teleport/teleportContext';

import { ResourceKind } from '../ResourceKind';
import { useUserTraits } from './useUserTraits';

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
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Kubernetes),
    };
    const spyUpdateAgentMeta = jest
      .spyOn(discoverCtx, 'updateAgentMeta')
      .mockImplementation(x => x);

    const { result } = renderHook(() => useUserTraits(), {
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

  test('kubernetes with auto discover preserves existing + new dynamic traits', async () => {
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Kubernetes),
    });
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Kubernetes),
      autoDiscovery: {
        config: { name: '', discoveryGroup: '', aws: [] },
        requiredVpcsAndSubnets: {},
      },
    };

    const { result } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.dynamicTraits.kubeUsers).toHaveLength(2)
    );

    expect(result.current.dynamicTraits.kubeGroups).toHaveLength(2);

    // Should not be setting statics.
    expect(result.current.staticTraits.kubeUsers).toHaveLength(0);
    expect(result.current.staticTraits.kubeGroups).toHaveLength(0);

    const addedTraitsOpts = {
      kubeUsers: [
        {
          isFixed: false,
          label: 'dynamicKbUser3',
          value: 'dynamicKbUser3',
        },
        {
          isFixed: false,
          label: 'dynamicKbUser4',
          value: 'dynamicKbUser4',
        },
        // duplicate
        {
          isFixed: false,
          label: 'dynamicKbUser4',
          value: 'dynamicKbUser4',
        },
      ],
      kubeGroups: [
        {
          isFixed: false,
          label: 'dynamicKbGroup3',
          value: 'dynamicKbGroup3',
        },
        {
          isFixed: false,
          label: 'dynamicKbGroup4',
          value: 'dynamicKbGroup4',
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
          kubeGroups: ['dynamicKbGroup3', 'dynamicKbGroup4'],
          kubeUsers: ['dynamicKbUser3', 'dynamicKbUser4'],
        },
      },
      ExcludeUserField.AllTraits
    );
  });

  test('database', async () => {
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Database),
    });
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Database),
    };
    const spyUpdateAgentMeta = jest
      .spyOn(discoverCtx, 'updateAgentMeta')
      .mockImplementation(x => x);

    const { result } = renderHook(() => useUserTraits(), {
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
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Database),
      autoDiscovery: {
        config: { name: '', discoveryGroup: '', aws: [] },
        requiredVpcsAndSubnets: {},
      },
    };

    const { result } = renderHook(() => useUserTraits(), {
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
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Server),
    };
    const spyUpdateAgentMeta = jest
      .spyOn(discoverCtx, 'updateAgentMeta')
      .mockImplementation(x => x);

    const { result } = renderHook(() => useUserTraits(), {
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

  test('only update user traits with dynamic awsRoleArns', async () => {
    const staticAwsRoles: AwsRole[] = [
      {
        name: 'static-arn1',
        arn: 'arn:aws:iam::123456789012:role/static-arn1',
        display: 'static-arn1',
        accountId: '123456789012',
      },
      {
        name: 'static-arn2',
        arn: 'arn:aws:iam::123456789012:role/static-arn2',
        display: 'static-arn2',
        accountId: '123456789012',
      },
    ];
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: {
        ...defaultResourceSpec(ResourceKind.Application),
        appMeta: { awsConsole: true },
      },
      agentMeta: {
        app: {
          ...app,
          awsRoles: staticAwsRoles,
        },
      },
    });
    discoverCtx.nextStep = jest.fn();
    const spyUpdateAgentMeta = jest
      .spyOn(discoverCtx, 'updateAgentMeta')
      .mockImplementation(x => x);

    const { result } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.staticTraits.awsRoleArns.length).toBeGreaterThan(0)
    );

    const dynamicTraits = result.current.dynamicTraits;
    const staticTraits = result.current.staticTraits;
    const mockedSelectedOptions = {
      awsRoleArns: [
        {
          isFixed: false,
          label: staticTraits.awsRoleArns[0],
          value: staticTraits.awsRoleArns[0],
        },
        {
          isFixed: false,
          label: dynamicTraits.awsRoleArns[0],
          value: dynamicTraits.awsRoleArns[0],
        },
        // duplicate
        {
          isFixed: false,
          label: dynamicTraits.awsRoleArns[0],
          value: dynamicTraits.awsRoleArns[0],
        },
      ],
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
        traits: {
          ...mockUser.traits,
          awsRoleArns: [dynamicTraits.awsRoleArns[0]],
        },
      },
      ExcludeUserField.AllTraits
    );

    // Test that app's awsRoles field got updated with the dynamic trait.
    const updatedMeta = spyUpdateAgentMeta.mock.results[0].value as AppMeta;
    expect(updatedMeta.app.awsRoles).toStrictEqual([
      ...staticAwsRoles,
      {
        name: 'dynamicArn1',
        display: 'dynamicArn1',
        arn: 'arn:aws:iam::123456789012:role/dynamicArn1',
        accountId: '123456789012',
      },
    ]);
  });

  test('node with auto discover preserves existing + new dynamic traits', async () => {
    const discoverCtx = defaultDiscoverContext({
      resourceSpec: defaultResourceSpec(ResourceKind.Server),
    });
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(ResourceKind.Server),
      autoDiscovery: {
        config: { name: '', discoveryGroup: '', aws: [] },
        requiredVpcsAndSubnets: {},
      },
    };

    const { result } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.dynamicTraits.logins).toHaveLength(2)
    );

    // Should not be setting statics.
    expect(result.current.staticTraits.logins).toHaveLength(0);

    const addedTraitsOpts = {
      logins: [
        {
          isFixed: true,
          label: 'banana',
          value: 'banana',
        },
        {
          isFixed: false,
          label: 'carrot',
          value: 'carrot',
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
          logins: ['banana', 'carrot'],
        },
      },
      ExcludeUserField.AllTraits
    );
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
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta = {
      ...discoverCtx.agentMeta,
      ...getMeta(resourceKind),
    };

    const { result } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });

    await waitFor(() =>
      expect(result.current.dynamicTraits.logins.length).toBeGreaterThan(0)
    );
    expect(teleCtx.userService.fetchUser).toHaveBeenCalled();
    expect(result.current.dynamicTraits.kubeGroups.length).toBeGreaterThan(0);
    expect(result.current.dynamicTraits.kubeUsers.length).toBeGreaterThan(0);
    expect(result.current.dynamicTraits.databaseNames.length).toBeGreaterThan(
      0
    );
    expect(result.current.dynamicTraits.databaseUsers.length).toBeGreaterThan(
      0
    );

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
    discoverCtx.nextStep = jest.fn();
    discoverCtx.agentMeta.autoDiscovery = {
      config: { name: '', discoveryGroup: '', aws: [] },
    };

    const { result } = renderHook(() => useUserTraits(), {
      wrapper: wrapperFn(discoverCtx, teleCtx),
    });
    await waitFor(() =>
      expect(result.current.dynamicTraits.logins.length).toBeGreaterThan(0)
    );

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
      kubeGroups: ['dynamicKbGroup1', 'dynamicKbGroup2'],
      windowsLogins: [],
      awsRoleArns: [
        'arn:aws:iam::123456789012:role/dynamicArn1',
        'arn:aws:iam::123456789012:role/dynamicArn2',
      ],
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

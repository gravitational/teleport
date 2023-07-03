/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { MemoryRouter } from 'react-router';
import { renderHook, act } from '@testing-library/react-hooks';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import {
  DiscoverProvider,
  DiscoverContextState,
} from 'teleport/Discover/useDiscover';
import api from 'teleport/services/api';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { userEventService } from 'teleport/services/userEvent';
import cfg from 'teleport/config';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';

import {
  useCreateDatabase,
  findActiveDatabaseSvc,
  WAITING_TIMEOUT,
} from './useCreateDatabase';

import type { CreateDatabaseRequest } from 'teleport/services/databases';

const dbLabels = [
  { name: 'env', value: 'prod' },
  { name: 'os', value: 'mac' },
  { name: 'tag', value: 'v11.0.0' },
];

const emptyAwsIdentity = {
  accountId: '',
  arn: '',
  resourceType: '',
  resourceName: '',
};

const services = [
  {
    name: 'svc1',
    matcherLabels: { os: ['windows', 'mac'], env: ['staging'] },
    awsIdentity: emptyAwsIdentity,
  },
  {
    name: 'svc2', // match
    matcherLabels: {
      os: ['windows', 'mac', 'linux'],
      tag: ['v11.0.0'],
      env: ['staging', 'prod'],
    },
    awsIdentity: emptyAwsIdentity,
  },
  {
    name: 'svc3',
    matcherLabels: { env: ['prod'], fruit: ['orange'] },
    awsIdentity: emptyAwsIdentity,
  },
];

const testCases = [
  {
    name: 'match with a service',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc4',
        matcherLabels: { env: ['prod'] },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: 'svc4',
  },
  {
    name: 'match among multple service',
    newLabels: dbLabels,
    services,
    expectedMatch: 'svc2',
  },
  {
    name: 'no match despite matching all labels when a svc has a non-matching label',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc2',
        matcherLabels: {
          os: ['windows', 'mac', 'linux'],
          fruit: ['apple', '*'], // the non-matching label
        },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: undefined,
  },
  {
    name: 'match by all asteriks',
    newLabels: [],
    services: [
      {
        name: 'svc1',
        matcherLabels: { '*': ['dev'], env: ['*'] },
        awsIdentity: emptyAwsIdentity,
      },
      {
        name: 'svc2',
        matcherLabels: { '*': ['*'] },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: 'svc2',
  },
  {
    name: 'match by asteriks, despite labels being defined',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc1',
        matcherLabels: { id: ['env', 'dev'], a: [], '*': ['*'] },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: 'svc1',
  },
  {
    name: 'match by any key, matching its val',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc1',
        matcherLabels: { env: ['*'], '*': ['dev'] },
        awsIdentity: emptyAwsIdentity,
      },
      {
        name: 'svc2',
        matcherLabels: {
          os: ['linux', 'mac'],
          '*': ['prod', 'apple'],
        },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: 'svc2',
  },
  {
    name: 'no matching value for any key',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc1',
        matcherLabels: { '*': ['windows'] },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: undefined,
  },
  {
    name: 'match by any val, matching its key',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc1',
        matcherLabels: {
          env: ['dev', '*'],
          os: ['windows', 'mac'],
          tag: ['*'],
        },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: 'svc1',
  },
  {
    name: 'no matching key for any value',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc1',
        matcherLabels: {
          fruit: ['*'],
          os: ['windows'],
        },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: undefined,
  },
  {
    name: 'no match',
    newLabels: dbLabels,
    services: [
      {
        name: 'svc1',
        matcherLabels: {
          fruit: ['*'],
        },
        awsIdentity: emptyAwsIdentity,
      },
    ],
    expectedMatch: undefined,
  },
  {
    name: 'no match with empty service list',
    newLabels: dbLabels,
    services: [],
    expectedMatch: undefined,
  },
  {
    name: 'no match with empty label fields',
    newLabels: dbLabels,
    services: [{ name: '', matcherLabels: {}, awsIdentity: emptyAwsIdentity }],
    expectedMatch: undefined,
  },
];

describe('findActiveDatabaseSvc()', () => {
  test.each(testCases)('$name', ({ newLabels, services, expectedMatch }) => {
    const foundSvc = findActiveDatabaseSvc(newLabels, services);
    expect(foundSvc?.name).toEqual(expectedMatch);
  });
});

const newDatabaseReq: CreateDatabaseRequest = {
  name: 'db-name',
  protocol: 'postgres',
  uri: 'https://localhost:5432',
  labels: dbLabels,
};

jest.useFakeTimers();

describe('registering new databases, mainly error checking', () => {
  const discoverCtx: DiscoverContextState = {
    agentMeta: {} as any,
    currentStep: 0,
    nextStep: jest.fn(x => x),
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      dbMeta: {
        location: DatabaseLocation.Aws,
        engine: DatabaseEngine.AuroraMysql,
      },
    } as any,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: jest.fn(x => x),
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };
  const teleCtx = createTeleportContext();

  let wrapper;

  beforeEach(() => {
    jest.spyOn(api, 'get').mockResolvedValue([]); // required for fetchClusterAlerts

    jest
      .spyOn(userEventService, 'captureDiscoverEvent')
      .mockResolvedValue(null as never); // return value does not matter but required by ts
    jest.spyOn(teleCtx.databaseService, 'fetchDatabases').mockResolvedValue({
      agents: [{ name: 'new-db', labels: dbLabels } as any],
    });
    jest
      .spyOn(teleCtx.databaseService, 'createDatabase')
      .mockResolvedValue(null); // ret val not used
    jest
      .spyOn(teleCtx.databaseService, 'updateDatabase')
      .mockResolvedValue(null); // ret val not used
    jest
      .spyOn(teleCtx.databaseService, 'fetchDatabaseServices')
      .mockResolvedValue({ services });

    wrapper = ({ children }) => (
      <MemoryRouter
        initialEntries={[
          { pathname: cfg.routes.discover, state: { entity: 'database' } },
        ]}
      >
        <ContextProvider ctx={teleCtx}>
          <FeaturesContextProvider value={[]}>
            <DiscoverProvider mockCtx={discoverCtx}>
              {children}
            </DiscoverProvider>
          </FeaturesContextProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('with matching service, activates polling', async () => {
    const { result } = renderHook(() => useCreateDatabase(), {
      wrapper,
    });

    // Check polling hasn't started.
    expect(teleCtx.databaseService.fetchDatabases).not.toHaveBeenCalled();

    await act(async () => {
      result.current.registerDatabase(newDatabaseReq);
    });
    expect(teleCtx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );

    await act(async () => jest.advanceTimersByTime(3000));
    expect(teleCtx.databaseService.fetchDatabases).toHaveBeenCalledTimes(1);
    expect(discoverCtx.updateAgentMeta).toHaveBeenCalledWith({
      resourceName: 'db-name',
      agentMatcherLabels: dbLabels,
      db: { name: 'new-db', labels: dbLabels },
    });

    // Test the dynamic definition of nextStep is called with a number
    // of steps to skip.
    result.current.nextStep();
    expect(discoverCtx.nextStep).toHaveBeenCalledWith(2);
  });

  test('when there are no services, skips polling', async () => {
    jest
      .spyOn(teleCtx.databaseService, 'fetchDatabaseServices')
      .mockResolvedValue({ services: [] } as any);
    const { result, waitFor } = renderHook(() => useCreateDatabase(), {
      wrapper,
    });

    act(() => {
      result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
    });

    await waitFor(() => {
      expect(teleCtx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(
        teleCtx.databaseService.fetchDatabaseServices
      ).toHaveBeenCalledTimes(1);
    });
    expect(discoverCtx.updateAgentMeta).toHaveBeenCalledWith({
      resourceName: 'db-name',
      agentMatcherLabels: [],
    });
    expect(teleCtx.databaseService.fetchDatabases).not.toHaveBeenCalled();

    // Test the dynamic definition of nextStep is called without
    // number of steps to skip defined.
    result.current.nextStep();
    expect(discoverCtx.nextStep).toHaveBeenCalledWith();
  });

  test('when failed to create db, stops flow', async () => {
    jest
      .spyOn(teleCtx.databaseService, 'createDatabase')
      .mockRejectedValue(null);
    const { result } = renderHook(() => useCreateDatabase(), {
      wrapper,
    });

    await act(async () => {
      result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
    });
    expect(teleCtx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
    expect(teleCtx.databaseService.fetchDatabases).not.toHaveBeenCalled();
    expect(discoverCtx.nextStep).not.toHaveBeenCalled();
    expect(result.current.attempt.status).toBe('failed');
  });

  test('when failed to fetch services, stops flow and retries properly', async () => {
    jest
      .spyOn(teleCtx.databaseService, 'fetchDatabaseServices')
      .mockRejectedValue(null);
    const { result } = renderHook(() => useCreateDatabase(), {
      wrapper,
    });

    await act(async () => {
      result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
    });

    expect(teleCtx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );
    expect(teleCtx.databaseService.fetchDatabases).not.toHaveBeenCalled();
    expect(discoverCtx.nextStep).not.toHaveBeenCalled();
    expect(result.current.attempt.status).toBe('failed');

    // Test retrying with same request, skips creating database since it's been already created.
    jest.clearAllMocks();
    await act(async () => {
      result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
    });
    expect(teleCtx.databaseService.createDatabase).not.toHaveBeenCalled();
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );
    expect(result.current.attempt.status).toBe('failed');

    // Test retrying with updated field, triggers create database.
    jest.clearAllMocks();
    await act(async () => {
      result.current.registerDatabase({
        ...newDatabaseReq,
        labels: [],
        uri: 'diff-uri',
      });
    });
    expect(teleCtx.databaseService.createDatabase).not.toHaveBeenCalled();
    expect(teleCtx.databaseService.updateDatabase).toHaveBeenCalledTimes(1);
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );
    expect(result.current.attempt.status).toBe('failed');
  });

  test('when polling timeout, retries properly', async () => {
    jest
      .spyOn(teleCtx.databaseService, 'fetchDatabases')
      .mockResolvedValue({ agents: [] });
    const { result } = renderHook(() => useCreateDatabase(), {
      wrapper,
    });

    await act(async () => {
      result.current.registerDatabase(newDatabaseReq);
    });

    act(() => jest.advanceTimersByTime(WAITING_TIMEOUT + 1));

    expect(teleCtx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );
    expect(teleCtx.databaseService.fetchDatabases).toHaveBeenCalled();
    expect(discoverCtx.nextStep).not.toHaveBeenCalled();
    expect(result.current.attempt.status).toBe('failed');
    expect(result.current.attempt.statusText).toContain('could not detect');

    // Test retrying with same request, skips creating database.
    jest.clearAllMocks();
    await act(async () => {
      result.current.registerDatabase(newDatabaseReq);
    });
    act(() => jest.advanceTimersByTime(WAITING_TIMEOUT + 1));

    expect(teleCtx.databaseService.createDatabase).not.toHaveBeenCalled();
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );
    expect(teleCtx.databaseService.fetchDatabases).toHaveBeenCalled();
    expect(result.current.attempt.status).toBe('failed');

    // Test retrying with request with updated fields, updates db and fetches new services.
    jest.clearAllMocks();
    await act(async () => {
      result.current.registerDatabase({
        ...newDatabaseReq,
        uri: 'diff-uri',
      });
    });
    act(() => jest.advanceTimersByTime(WAITING_TIMEOUT + 1));

    expect(teleCtx.databaseService.updateDatabase).toHaveBeenCalledTimes(1);
    expect(teleCtx.databaseService.createDatabase).not.toHaveBeenCalled();
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );
    expect(teleCtx.databaseService.fetchDatabases).toHaveBeenCalled();
    expect(result.current.attempt.status).toBe('failed');
  });
});

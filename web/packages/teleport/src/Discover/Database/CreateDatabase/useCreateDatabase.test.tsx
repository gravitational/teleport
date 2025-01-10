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

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import api from 'teleport/services/api';
import {
  CreateDatabaseRequest,
  IamPolicyStatus,
} from 'teleport/services/databases';
import { userEventService } from 'teleport/services/userEvent';

import {
  findActiveDatabaseSvc,
  useCreateDatabase,
  WAITING_TIMEOUT,
} from './useCreateDatabase';

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
const defaultIsCloud = cfg.isCloud;

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
    cfg.isCloud = true;
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
    cfg.isCloud = defaultIsCloud;
    jest.clearAllMocks();
  });

  test('polling until result returns (non aws)', async () => {
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
    cfg.isCloud = false;
    result.current.nextStep();
    expect(discoverCtx.nextStep).toHaveBeenCalledWith(3);
  });

  test('continue polling when poll result returns with iamPolicyStatus field set to "pending"', async () => {
    jest.spyOn(teleCtx.databaseService, 'fetchDatabases').mockResolvedValue({
      agents: [
        {
          name: 'new-db',
          aws: { iamPolicyStatus: IamPolicyStatus.Pending },
        } as any,
      ],
    });
    const { result } = renderHook(() => useCreateDatabase(), {
      wrapper,
    });

    await act(async () => {
      result.current.registerDatabase(newDatabaseReq);
    });
    expect(teleCtx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
    expect(teleCtx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
      1
    );

    // The first result will not have the aws marker we are looking for.
    // Polling should continue.
    await act(async () => jest.advanceTimersByTime(3000));
    expect(teleCtx.databaseService.fetchDatabases).toHaveBeenCalledTimes(1);
    expect(discoverCtx.updateAgentMeta).not.toHaveBeenCalled();

    // Set the marker we are looking for in the next api reply.
    jest.clearAllMocks();
    jest.spyOn(teleCtx.databaseService, 'fetchDatabases').mockResolvedValue({
      agents: [
        {
          name: 'new-db',
          aws: { iamPolicyStatus: IamPolicyStatus.Success },
        } as any,
      ],
    });

    // The second poll result has the marker that should cancel polling.
    await act(async () => jest.advanceTimersByTime(3000));
    expect(teleCtx.databaseService.fetchDatabases).toHaveBeenCalledTimes(1);
    expect(discoverCtx.updateAgentMeta).toHaveBeenCalledWith({
      resourceName: 'db-name',
      db: {
        name: 'new-db',
        aws: { iamPolicyStatus: IamPolicyStatus.Success },
      },
      serviceDeployedMethod: 'skipped',
    });

    result.current.nextStep();
    // Skips both deploy service AND IAM policy step.
    expect(discoverCtx.nextStep).toHaveBeenCalledWith(3);
    cfg.isCloud = false;
    result.current.nextStep();
    expect(discoverCtx.nextStep).toHaveBeenCalledWith(4);
  });

  test('stops polling when poll result returns with iamPolicyStatus field set to "unspecified"', async () => {
    jest.spyOn(teleCtx.databaseService, 'fetchDatabases').mockResolvedValue({
      agents: [
        {
          name: 'new-db',
          aws: { iamPolicyStatus: IamPolicyStatus.Unspecified },
        } as any,
      ],
    });
    const { result } = renderHook(() => useCreateDatabase(), {
      wrapper,
    });

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
      db: {
        name: 'new-db',
        aws: { iamPolicyStatus: IamPolicyStatus.Unspecified },
      },
    });

    result.current.nextStep();
    expect(discoverCtx.nextStep).toHaveBeenCalledWith(2);
  });

  test('when there are no services, skips polling', async () => {
    jest
      .spyOn(teleCtx.databaseService, 'fetchDatabaseServices')
      .mockResolvedValue({ services: [] } as any);
    const { result } = renderHook(() => useCreateDatabase(), {
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
    cfg.isCloud = false;
    result.current.nextStep();
    expect(discoverCtx.nextStep).toHaveBeenCalledWith(2);
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

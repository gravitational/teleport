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

// import React from 'react';
// import { renderHook, act } from '@testing-library/react-hooks';

// import { createTeleportContext } from 'teleport/mocks/contexts';
// import { ContextProvider } from 'teleport';

import {
  // useCreateDatabase,
  findActiveDatabaseSvc,
  // WAITING_TIMEOUT,
} from './useCreateDatabase';

// import type { CreateDatabaseRequest } from 'teleport/services/databases';

const dbLabels = [
  { name: 'env', value: 'prod' },
  { name: 'os', value: 'mac' },
  { name: 'tag', value: 'v11.0.0' },
];

const services = [
  {
    name: 'svc1',
    matcherLabels: { os: ['windows', 'linux'], env: ['staging'] },
  },
  { name: 'svc2', matcherLabels: { fruit: ['orange'] } },
  { name: 'svc3', matcherLabels: { os: ['windows', 'mac'] } }, // match
];

describe('findActiveDatabaseSvc', () => {
  test.each`
    desc                                       | dbLabels                     | services                                                   | expected
    ${'match with multi elements'}             | ${dbLabels}                  | ${services}                                                | ${true}
    ${'match by asteriks'}                     | ${[]}                        | ${[{ matcherLabels: { '*': ['*'] } }]}                     | ${true}
    ${'match by asteriks with labels defined'} | ${dbLabels}                  | ${[{ matcherLabels: { id: ['123', '123'], '*': ['*'] } }]} | ${true}
    ${'match by any key, matching val'}        | ${dbLabels}                  | ${[{ matcherLabels: { '*': ['windows', 'mac'] } }]}        | ${true}
    ${'match by any key, no matching val'}     | ${dbLabels}                  | ${[{ matcherLabels: { '*': ['windows', 'linux'] } }]}      | ${false}
    ${'match by any val, matching key'}        | ${dbLabels}                  | ${[{ matcherLabels: { test: ['*'], tag: ['*'] } }]}        | ${true}
    ${'match by any val, no matching key'}     | ${dbLabels}                  | ${[{ matcherLabels: { test: ['*'], test2: ['*'] } }]}      | ${false}
    ${'no match'}                              | ${dbLabels}                  | ${[{ matcherLabels: { os: ['linux', 'windows'] } }]}       | ${false}
    ${'no match with empty lists'}             | ${[]}                        | ${[]}                                                      | ${false}
    ${'no match with empty fields'}            | ${[{ name: '', value: '' }]} | ${[{ matcherLabels: {} }]}                                 | ${false}
    ${'no match with any key'}                 | ${[]}                        | ${[{ matcherLabels: { '*': ['mac'] } }]}                   | ${false}
    ${'no match with any val'}                 | ${[]}                        | ${[{ matcherLabels: { os: ['*'] } }]}                      | ${false}
  `('$desc', ({ dbLabels, services, expected }) => {
    expect(findActiveDatabaseSvc(dbLabels, services)).toEqual(expected);
  });
});

// const newDatabaseReq: CreateDatabaseRequest = {
//   name: 'db-name',
//   protocol: 'postgres',
//   uri: 'https://localhost:5432',
//   labels: dbLabels,
// };

// jest.useFakeTimers();

// eslint-disable-next-line jest/no-commented-out-tests
// describe('registering new databases, mainly error checking', () => {
//   const props = {
//     agentMeta: {} as any,
//     updateAgentMeta: jest.fn(x => x),
//     nextStep: jest.fn(x => x),
//     resourceState: {},
//   };
//   const ctx = createTeleportContext();

//   let wrapper;

//   beforeEach(() => {
//     jest
//       .spyOn(ctx.databaseService, 'fetchDatabases')
//       .mockResolvedValue({ agents: [{ name: 'new-db' } as any] });
//     jest.spyOn(ctx.databaseService, 'createDatabase').mockResolvedValue(null); // ret val not used
//     jest
//       .spyOn(ctx.databaseService, 'fetchDatabaseServices')
//       .mockResolvedValue({ services });

//     wrapper = ({ children }) => (
//       <ContextProvider ctx={ctx}>{children}</ContextProvider>
//     );
//   });

//   afterEach(() => {
//     jest.clearAllMocks();
//   });

// eslint-disable-next-line jest/no-commented-out-tests
//   test('with matching service, activates polling', async () => {
//     const { result } = renderHook(() => useCreateDatabase(props), {
//       wrapper,
//     });

//     // Check polling hasn't started.
//     expect(ctx.databaseService.fetchDatabases).not.toHaveBeenCalled();

//     await act(async () => {
//       result.current.registerDatabase(newDatabaseReq);
//     });
//     expect(ctx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(1);

//     await act(async () => jest.advanceTimersByTime(3000));
//     expect(ctx.databaseService.fetchDatabases).toHaveBeenCalledTimes(1);
//     expect(props.nextStep).toHaveBeenCalledWith(2);
//     expect(props.updateAgentMeta).toHaveBeenCalledWith({
//       resourceName: 'db-name',
//       agentMatcherLabels: dbLabels,
//       db: { name: 'new-db' },
//     });
//   });

// eslint-disable-next-line jest/no-commented-out-tests
//   test('when there are no services, skips polling', async () => {
//     jest
//       .spyOn(ctx.databaseService, 'fetchDatabaseServices')
//       .mockResolvedValue({ services: [] } as any);
//     const { result, waitFor } = renderHook(() => useCreateDatabase(props), {
//       wrapper,
//     });

//     act(() => {
//       result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
//     });

//     await waitFor(() => {
//       expect(ctx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
//     });

//     await waitFor(() => {
//       expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(
//         1
//       );
//     });

//     expect(props.nextStep).toHaveBeenCalledWith();
//     expect(props.updateAgentMeta).toHaveBeenCalledWith({
//       resourceName: 'db-name',
//       agentMatcherLabels: [],
//     });
//     expect(ctx.databaseService.fetchDatabases).not.toHaveBeenCalled();
//   });

// eslint-disable-next-line jest/no-commented-out-tests
//   test('when failed to create db, stops flow', async () => {
//     jest.spyOn(ctx.databaseService, 'createDatabase').mockRejectedValue(null);
//     const { result } = renderHook(() => useCreateDatabase(props), {
//       wrapper,
//     });

//     await act(async () => {
//       result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
//     });

//     expect(ctx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabases).not.toHaveBeenCalled();
//     expect(props.nextStep).not.toHaveBeenCalled();
//     expect(result.current.attempt.status).toBe('failed');
//   });

// eslint-disable-next-line jest/no-commented-out-tests
//   test('when failed to fetch services, stops flow and retries properly', async () => {
//     jest
//       .spyOn(ctx.databaseService, 'fetchDatabaseServices')
//       .mockRejectedValue(null);
//     const { result } = renderHook(() => useCreateDatabase(props), {
//       wrapper,
//     });

//     await act(async () => {
//       result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
//     });

//     expect(ctx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabases).not.toHaveBeenCalled();
//     expect(props.nextStep).not.toHaveBeenCalled();
//     expect(result.current.attempt.status).toBe('failed');

//     // Test retrying with same request, skips creating database since it's been already created.
//     jest.clearAllMocks();
//     await act(async () => {
//       result.current.registerDatabase({ ...newDatabaseReq, labels: [] });
//     });
//     expect(ctx.databaseService.createDatabase).not.toHaveBeenCalled();
//     expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(1);
//     expect(result.current.attempt.status).toBe('failed');

//     // Test retrying with a new db request (new name), triggers create database.
//     jest.clearAllMocks();
//     await act(async () => {
//       result.current.registerDatabase({
//         ...newDatabaseReq,
//         labels: [],
//         name: 'new-db-name',
//       });
//     });
//     expect(ctx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(1);
//     expect(result.current.attempt.status).toBe('failed');
//   });

// eslint-disable-next-line jest/no-commented-out-tests
//   test('when polling timeout, retries properly', async () => {
//     jest
//       .spyOn(ctx.databaseService, 'fetchDatabases')
//       .mockResolvedValue({ agents: [] });
//     const { result } = renderHook(() => useCreateDatabase(props), {
//       wrapper,
//     });

//     await act(async () => {
//       result.current.registerDatabase(newDatabaseReq);
//     });

//     act(() => jest.advanceTimersByTime(WAITING_TIMEOUT + 1));

//     expect(ctx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabases).toHaveBeenCalled();
//     expect(props.nextStep).not.toHaveBeenCalled();
//     expect(result.current.attempt.status).toBe('failed');
//     expect(result.current.attempt.statusText).toContain('could not detect');

//     // Test retrying with same request, skips creating database.
//     jest.clearAllMocks();
//     await act(async () => {
//       result.current.registerDatabase(newDatabaseReq);
//     });
//     act(() => jest.advanceTimersByTime(WAITING_TIMEOUT + 1));

//     expect(ctx.databaseService.createDatabase).not.toHaveBeenCalled();
//     expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabases).toHaveBeenCalled();
//     expect(result.current.attempt.status).toBe('failed');

//     // Test retrying with request with diff db name, creates and fetches new services.
//     jest.clearAllMocks();
//     await act(async () => {
//       result.current.registerDatabase({
//         ...newDatabaseReq,
//         name: 'new-db-name',
//       });
//     });
//     act(() => jest.advanceTimersByTime(WAITING_TIMEOUT + 1));

//     expect(ctx.databaseService.createDatabase).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabaseServices).toHaveBeenCalledTimes(1);
//     expect(ctx.databaseService.fetchDatabases).toHaveBeenCalled();
//     expect(result.current.attempt.status).toBe('failed');
//   });
// });

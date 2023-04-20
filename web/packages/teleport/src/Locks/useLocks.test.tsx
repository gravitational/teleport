/*
Copyright 2023 Gravitational, Inc.

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

import { renderHook } from '@testing-library/react-hooks';

import { useLocks } from './useLocks';

import {
  HOOK_LIST as mockHookList,
  HOOK_CREATED as mockHookCreated,
} from './testFixtures';

jest.mock('teleport/services/api', () => ({
  get: () => new Promise(resolve => resolve(mockHookList)),
  put: () => new Promise(resolve => resolve(mockHookCreated)),
}));

describe('hook: useLocks', () => {
  it('fetches and returns the locks', async () => {
    const { result, waitForNextUpdate } = renderHook(() =>
      useLocks('cluster-id')
    );
    result.current.fetchLocks('cluster-id');
    expect(result.current.locks).toHaveLength(0);
    await waitForNextUpdate();
    expect(result.current.locks).toHaveLength(4);
  });

  it('creates locks', async () => {
    const { result, waitForNextUpdate } = renderHook(() =>
      useLocks('cluster-id')
    );
    // When the hook is initialized it fetches all hooks so wait for this to
    // happen before continuing on.
    await waitForNextUpdate();
    const resp = await result.current.createLock('cluster-id', {
      targets: { user: 'banned' },
      message: "you've been bad",
      ttl: '5h',
    });
    expect(resp).toBe(mockHookCreated);
  });
});

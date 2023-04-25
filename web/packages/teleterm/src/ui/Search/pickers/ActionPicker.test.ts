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

import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { ResourceSearchError } from 'teleterm/ui/services/resources';

import { getActionPickerStatus } from './ActionPicker';

describe('getActionPickerStatus', () => {
  it('partitions resource search errors into clusters with expired certs and non-retryable errors', () => {
    const retryableError = new ResourceSearchError(
      '/clusters/foo',
      'server',
      new Error('ssh: cert has expired')
    );

    const nonRetryableError = new ResourceSearchError(
      '/clusters/bar',
      'database',
      new Error('whoops')
    );

    const status = getActionPickerStatus({
      inputValue: 'foo',
      filters: [],
      allClusters: [],
      actionAttempts: [makeSuccessAttempt([])],
      resourceSearchAttempt: makeSuccessAttempt({
        errors: [retryableError, nonRetryableError],
        results: [],
        search: 'foo',
      }),
    });

    expect(status.status).toBe('finished');

    const { clustersWithExpiredCerts, nonRetryableResourceSearchErrors } =
      status.status === 'finished' && status;

    expect([...clustersWithExpiredCerts]).toEqual([retryableError.clusterUri]);
    expect(nonRetryableResourceSearchErrors).toEqual([nonRetryableError]);
  });

  it('merges non-connected clusters with clusters that returned retryable errors', () => {
    const offlineCluster = makeRootCluster({ connected: false });
    const retryableError = new ResourceSearchError(
      '/clusters/foo',
      'server',
      new Error('ssh: cert has expired')
    );

    const status = getActionPickerStatus({
      inputValue: 'foo',
      filters: [],
      allClusters: [offlineCluster],
      actionAttempts: [makeSuccessAttempt([])],
      resourceSearchAttempt: makeSuccessAttempt({
        errors: [retryableError],
        results: [],
        search: 'foo',
      }),
    });

    expect(status.status).toBe('finished');
    const { clustersWithExpiredCerts } = status.status === 'finished' && status;

    expect(clustersWithExpiredCerts.size).toBe(2);
    expect(clustersWithExpiredCerts).toContain(offlineCluster.uri);
    expect(clustersWithExpiredCerts).toContain(retryableError.clusterUri);
  });

  it('includes a cluster with expired cert only once even if multiple requests fail with retryable errors', () => {
    const retryableErrors = [
      new ResourceSearchError(
        '/clusters/foo',
        'server',
        new Error('ssh: cert has expired')
      ),
      new ResourceSearchError(
        '/clusters/foo',
        'database',
        new Error('ssh: cert has expired')
      ),
      new ResourceSearchError(
        '/clusters/foo',
        'kube',
        new Error('ssh: cert has expired')
      ),
    ];
    const status = getActionPickerStatus({
      inputValue: 'foo',
      filters: [],
      allClusters: [],
      actionAttempts: [makeSuccessAttempt([])],
      resourceSearchAttempt: makeSuccessAttempt({
        errors: retryableErrors,
        results: [],
        search: 'foo',
      }),
    });

    expect(status.status).toBe('finished');
    const { clustersWithExpiredCerts } = status.status === 'finished' && status;
    expect([...clustersWithExpiredCerts]).toEqual(['/clusters/foo']);
  });
});

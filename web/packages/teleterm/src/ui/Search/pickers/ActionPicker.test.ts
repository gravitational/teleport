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
  describe('some-input search mode', () => {
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
        filterActionsAttempt: makeSuccessAttempt([]),
        allClusters: [],
        actionAttempts: [makeSuccessAttempt([])],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: [retryableError, nonRetryableError],
          results: [],
          search: 'foo',
        }),
      });

      expect(status.inputState).toBe('some-input');

      const { clustersWithExpiredCerts, nonRetryableResourceSearchErrors } =
        status.inputState === 'some-input' && status;

      expect([...clustersWithExpiredCerts]).toEqual([
        retryableError.clusterUri,
      ]);
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
        filterActionsAttempt: makeSuccessAttempt([]),
        allClusters: [offlineCluster],
        actionAttempts: [makeSuccessAttempt([])],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: [retryableError],
          results: [],
          search: 'foo',
        }),
      });

      expect(status.inputState).toBe('some-input');
      const { clustersWithExpiredCerts } =
        status.inputState === 'some-input' && status;

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
        filterActionsAttempt: makeSuccessAttempt([]),
        allClusters: [],
        actionAttempts: [makeSuccessAttempt([])],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: retryableErrors,
          results: [],
          search: 'foo',
        }),
      });

      expect(status.inputState).toBe('some-input');
      const { clustersWithExpiredCerts } =
        status.inputState === 'some-input' && status;
      expect([...clustersWithExpiredCerts]).toEqual(['/clusters/foo']);
    });

    describe('when there are no results', () => {
      it('lists only the filtered offline cluster if a cluster filter is selected and the filtered cluster is offline', () => {
        const filteredCluster = makeRootCluster({
          connected: false,
          uri: '/clusters/filtered-cluster',
        });
        const otherOfflineCluster = makeRootCluster({
          connected: false,
          uri: '/clusters/other-offline-cluster',
        });
        const status = getActionPickerStatus({
          inputValue: 'foo',
          filters: [{ filter: 'cluster', clusterUri: filteredCluster.uri }],
          filterActionsAttempt: makeSuccessAttempt([]),
          allClusters: [filteredCluster, otherOfflineCluster],
          actionAttempts: [makeSuccessAttempt([])],
          resourceSearchAttempt: makeSuccessAttempt({
            errors: [],
            results: [],
            search: 'foo',
          }),
        });

        expect(status.inputState).toBe('some-input');
        const { clustersWithExpiredCerts } =
          status.inputState === 'some-input' && status;
        expect([...clustersWithExpiredCerts]).toEqual([filteredCluster.uri]);
      });

      it('does not list offline clusters if a cluster filter is selected and that cluster is online and there are no results', () => {
        const filteredCluster = makeRootCluster({
          connected: true,
          uri: '/clusters/filtered-cluster',
        });
        const otherOfflineCluster = makeRootCluster({
          connected: false,
          uri: '/clusters/other-offline-cluster',
        });
        const status = getActionPickerStatus({
          inputValue: 'foo',
          filters: [{ filter: 'cluster', clusterUri: filteredCluster.uri }],
          filterActionsAttempt: makeSuccessAttempt([]),
          allClusters: [filteredCluster, otherOfflineCluster],
          actionAttempts: [makeSuccessAttempt([])],
          resourceSearchAttempt: makeSuccessAttempt({
            errors: [],
            results: [],
            search: 'foo',
          }),
        });

        expect(status.inputState).toBe('some-input');
        const { clustersWithExpiredCerts } =
          status.inputState === 'some-input' && status;
        expect([...clustersWithExpiredCerts]).toHaveLength(0);
      });
    });
  });

  describe('no-input search mode', () => {
    it('returns non-retryable errors when fetching a preview after selecting a filter fails', () => {
      const nonRetryableError = new ResourceSearchError(
        '/clusters/bar',
        'server',
        new Error('non-retryable error')
      );
      const resourceSearchErrors = [
        new ResourceSearchError(
          '/clusters/foo',
          'server',
          new Error('ssh: cert has expired')
        ),
        nonRetryableError,
      ];
      const status = getActionPickerStatus({
        inputValue: '',
        filters: [{ filter: 'resource-type', resourceType: 'servers' }],
        filterActionsAttempt: makeSuccessAttempt([]),
        allClusters: [],
        actionAttempts: [makeSuccessAttempt([])],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: resourceSearchErrors,
          results: [],
          search: '',
        }),
      });

      expect(status.inputState).toBe('no-input');

      const { searchMode } = status.inputState === 'no-input' && status;
      expect(searchMode.kind).toBe('preview');

      const { nonRetryableResourceSearchErrors } =
        searchMode.kind === 'preview' && searchMode;
      expect(nonRetryableResourceSearchErrors).toEqual([nonRetryableError]);
    });
  });
});
